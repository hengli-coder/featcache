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

// Package featcache implements a zero-copy runtime data cache for AI inference.
//
// Architecture: single-writer (Loader) + multiple-reader (inference processes).
// The Loader owns the shared memory segment, writes data, and serves metadata
// lookups over Unix Domain Socket. Clients mmap the same segment and read data
// directly — zero-copy, no locks, no syscalls on the read path.
package featcache

// --- Segment layout constants ---

const (
	// Magic is the first 4 bytes of the segment header: "FEAT" little-endian.
	Magic = 0x46454154

	// Version is the current layout version.
	Version = 1

	// HeaderSize is the size of the segment header (one cache line).
	HeaderSize = 64

	// SlotSize is the size of each hash table slot (24 bytes).
	// Layout: [hash:8B][offset:4B][vlen:4B][status:4B][pad:4B].
	SlotSize = 24

	// SegmentDefaultSize is the default segment size (2 GB).
	SegmentDefaultSize = 2 << 30

	// MaxKeyLen is the maximum key length in bytes.
	MaxKeyLen = 256
)

// --- Header: the first 64 bytes of a shared memory segment ---
//
// All fields are stored in native byte order (little-endian on x86/ARM).
//
//	Offset  Size  Field
//	0       4     Magic
//	4       4     Version
//	8       8     Size
//	16      8     GenCounter
//	24      4     HashCap (number of slots, power of 2)
//	28      4     HashOffset (bytes from segment start)
//	32      4     DataOffset (bytes from segment start)
//	36      4     DataEnd (next free byte in data region, relative to segment start)
//	40      4     SegmentID
//	44      4     Flags (reserved)
//	48      16    Reserved
type Header struct {
	Magic      uint32
	Version    uint32
	Size       uint64
	GenCounter uint64
	HashCap    uint32
	HashOffset uint32
	DataOffset uint32
	DataEnd    uint32
	SegmentID  uint32
	Flags      uint32
	Reserved   [16]byte
}

// --- Slot status constants ---

const (
	SlotEmpty uint32 = 0 // Slot has never been written
	SlotUsed  uint32 = 1 // Slot holds a valid key-value pair
	SlotTomb  uint32 = 2 // Logical deletion; preserves probe sequence
)

// --- HashSlot: 24 bytes, stored in the hash table region ---
//
//	Offset  Size  Field
//	0       8     Hash (full 64-bit hash)
//	8       4     Offset (byte offset into data region, relative to DataOffset)
//	12      4     VLen (value length in bytes)
//	16      4     Status (SlotEmpty / SlotUsed / SlotTomb)
//	20      4     Reserved
type HashSlot struct {
	Hash   uint64 // full 64-bit hash — avoids key comparison on most lookups
	Offset uint32 // byte offset into data region (relative to DataOffset)
	VLen   uint32 // value length in bytes
	Status uint32 // SlotEmpty / SlotUsed / SlotTomb
	_      [4]byte
}

// --- OpCode for UDS protocol ---

type OpCode byte

const (
	OpGetInfo   OpCode = 0x01 // Get segment metadata
	OpGetStatus OpCode = 0x02 // Get loader status
	OpWatch     OpCode = 0x03 // Watch for version changes (Phase 2)
	OpPin       OpCode = 0x04 // Pin data in memory (Phase 3)
	OpPrefetch  OpCode = 0x05 // Prefetch data to cache (Phase 3)
	OpEvict     OpCode = 0x06 // Evict cache data (Phase 3)
	OpList      OpCode = 0x07 // List loaded datasets (Phase 3)
	OpReload    OpCode = 0x08 // Trigger reload (Phase 3)
)

// --- StatusCode for UDS responses ---

type StatusCode byte

const (
	RespOK       StatusCode = 0x00
	RespNotFound StatusCode = 0x01
	RespBusy     StatusCode = 0x02 // Loader is reloading
	RespError    StatusCode = 0x03
)

// Align returns the smallest value >= v that is a multiple of n.
func Align(v, n uint32) uint32 {
	return (v + n - 1) &^ (n - 1)
}

// NextPow2 returns the smallest power of 2 >= v.
func NextPow2(v uint32) uint32 {
	if v == 0 {
		return 1
	}
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v++
	return v
}
