// Copyright 2026 featcache contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package featcache

import (
	"encoding/binary"
	"sync/atomic"
	"unsafe"
)

// HashTable is an open-addressed, linear-probing hash table stored in shared memory.
//
// Single-writer / multiple-reader model:
//   - Writer: uses CAS to claim slots, atomic store to mark status
//   - Readers: atomic load to read status — no locks, no syscalls
//
// Slot layout (24 bytes):
//
//	[hash:8B][offset:4B][vlen:4B][status:4B][pad:4B]
//
// The writer stores data BEFORE marking the slot as SlotUsed (release store),
// so readers that observe SlotUsed will see the complete data.

// HashTable provides lookup/insert/delete over a flat slot array in shared memory.
type HashTable struct {
	data     []byte // the full segment (or a test buffer)
	slotBase int    // byte offset of first slot in data
	dataBase int    // byte offset where the data region starts in data
	capacity int    // number of slots (power of 2)
	mask     int    // capacity - 1
}

// NewHashTable creates a HashTable handle.
//
// Parameters:
//   - data: the backing byte slice (shared memory or in-memory for tests)
//   - slotBase: byte offset within data where the hash table starts
//   - dataBase: byte offset within data where the data region starts
//   - capacity: number of slots (must be a power of 2)
func NewHashTable(data []byte, slotBase, dataBase, capacity int) *HashTable {
	return &HashTable{
		data:     data,
		slotBase: slotBase,
		dataBase: dataBase,
		capacity: capacity,
		mask:     capacity - 1,
	}
}

// InitHashTable zeroes the slot region. Returns the byte offset past the table.
func InitHashTable(data []byte, slotBase int, numSlots int) int {
	slotBytes := numSlots * SlotSize
	for i := 0; i < slotBytes; i++ {
		data[slotBase+i] = 0
	}
	return slotBase + slotBytes
}

// getSlot reads slot idx atomically and returns a copy.
func (ht *HashTable) getSlot(idx int) HashSlot {
	off := ht.slotBase + idx*SlotSize
	status := atomic.LoadUint32((*uint32)(unsafe.Pointer(&ht.data[off+16])))
	return HashSlot{
		Hash:   binary.LittleEndian.Uint64(ht.data[off : off+8]),
		Offset: binary.LittleEndian.Uint32(ht.data[off+8 : off+12]),
		VLen:   binary.LittleEndian.Uint32(ht.data[off+12 : off+16]),
		Status: status,
	}
}

// Get looks up a key. Returns the value slice (backed by shared memory) and true,
// or nil and false if not found.
//
// The returned slice is a view into shared memory — callers must not modify it.
func (ht *HashTable) Get(hash uint64, key []byte) ([]byte, bool) {
	idx := int(uint32(hash)) & ht.mask

	for i := 0; i < ht.capacity; i++ {
		slot := ht.getSlot(idx)

		switch slot.Status {
		case SlotEmpty:
			return nil, false
		case SlotUsed:
			if slot.Hash == hash && ht.matchKeyAt(slot.Offset, key) {
				val := ht.getValue(slot.Offset, slot.VLen)
				return val, true
			}
		case SlotTomb:
			// continue probing
		}
		idx = (idx + 1) & ht.mask
	}
	return nil, false
}

// Insert inserts a new key-value pair into the table.
// Returns true on success, false if the table is full.
//
// offset and vlen are relative to DataOffset in the data region.
// The caller MUST write the key-value data to the data region BEFORE calling Insert.
func (ht *HashTable) Insert(hash uint64, _ []byte, offset, vlen uint32) bool {
	idx := int(uint32(hash)) & ht.mask

	for i := 0; i < ht.capacity; i++ {
		off := ht.slotBase + idx*SlotSize
		status := atomic.LoadUint32((*uint32)(unsafe.Pointer(&ht.data[off+16])))

		if status == SlotEmpty || status == SlotTomb {
			if !atomic.CompareAndSwapUint32((*uint32)(unsafe.Pointer(&ht.data[off+16])), status, SlotUsed) {
				idx = (idx + 1) & ht.mask
				continue
			}
			// Slot claimed — write data fields.
			binary.LittleEndian.PutUint64(ht.data[off:off+8], hash)
			binary.LittleEndian.PutUint32(ht.data[off+8:off+12], offset)
			binary.LittleEndian.PutUint32(ht.data[off+12:off+16], vlen)
			return true
		}

		idx = (idx + 1) & ht.mask
	}
	return false
}

// Delete removes a key by setting its slot to tombstone.
func (ht *HashTable) Delete(hash uint64, key []byte) bool {
	idx := int(uint32(hash)) & ht.mask

	for i := 0; i < ht.capacity; i++ {
		off := ht.slotBase + idx*SlotSize
		status := atomic.LoadUint32((*uint32)(unsafe.Pointer(&ht.data[off+16])))

		switch status {
		case SlotEmpty:
			return false
		case SlotUsed:
			if ht.slotHash(idx) == hash && ht.matchKeyAtSlot(idx, key) {
				atomic.StoreUint32((*uint32)(unsafe.Pointer(&ht.data[off+16])), SlotTomb)
				return true
			}
		}
		idx = (idx + 1) & ht.mask
	}
	return false
}

// matchKeyAt reads the key stored at data offset (relative to DataOffset)
// and compares it with the given key.
func (ht *HashTable) matchKeyAt(dataOff uint32, key []byte) bool {
	absOff := ht.dataBase + int(dataOff)
	if absOff+4 > len(ht.data) {
		return false
	}
	kLen := int(binary.LittleEndian.Uint32(ht.data[absOff : absOff+4]))
	if kLen != len(key) {
		return false
	}
	if absOff+4+kLen > len(ht.data) {
		return false
	}
	for i := 0; i < kLen; i++ {
		if ht.data[absOff+4+i] != key[i] {
			return false
		}
	}
	return true
}

// matchKeyAtSlot reads the key stored at the data offset referenced by slot idx.
func (ht *HashTable) matchKeyAtSlot(idx int, key []byte) bool {
	off := ht.slotBase + idx*SlotSize
	dataOff := binary.LittleEndian.Uint32(ht.data[off+8 : off+12])
	return ht.matchKeyAt(dataOff, key)
}

// slotHash returns the hash stored at slot idx.
func (ht *HashTable) slotHash(idx int) uint64 {
	off := ht.slotBase + idx*SlotSize
	return binary.LittleEndian.Uint64(ht.data[off : off+8])
}

// getValue reads a value from shared memory.
// Data layout at offset: [keyLen:4B][keyBytes:keyLen][valueBytes:vLen].
func (ht *HashTable) getValue(dataOff uint32, vLen uint32) []byte {
	absOff := ht.dataBase + int(dataOff)
	if absOff+4 > len(ht.data) {
		return nil
	}
	kLen := int(binary.LittleEndian.Uint32(ht.data[absOff : absOff+4]))
	valOff := absOff + 4 + kLen
	if valOff+int(vLen) > len(ht.data) {
		return nil
	}
	return ht.data[valOff : valOff+int(vLen)]
}

// SlotAt returns a copy of the slot at the given index.
func (ht *HashTable) SlotAt(idx int) HashSlot {
	return ht.getSlot(idx)
}

// Count returns the number of used slots (O(n)).
func (ht *HashTable) Count() int {
	c := 0
	for i := 0; i < ht.capacity; i++ {
		if ht.getSlot(i).Status == SlotUsed {
			c++
		}
	}
	return c
}

// Iterate calls fn for every used slot. If fn returns false, iteration stops.
func (ht *HashTable) Iterate(fn func(slot HashSlot) bool) {
	for i := 0; i < ht.capacity; i++ {
		slot := ht.getSlot(i)
		if slot.Status == SlotUsed {
			if !fn(slot) {
				return
			}
		}
	}
}
