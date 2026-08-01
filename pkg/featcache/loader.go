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
	"errors"
	"log"
	"sync/atomic"
)

// LoaderConfig configures the behavior of a Loader.
type LoaderConfig struct {
	// SegmentName is the shared memory segment name.
	SegmentName string

	// SegmentSize is the total segment size in bytes. Default: 2GB.
	SegmentSize int

	// LoadFactor is the target hash table load factor (0.0-1.0). Default: 0.5.
	// At load factor 0.5, half the slots are used and half are empty.
	LoadFactor float64
}

func (c *LoaderConfig) defaults() {
	if c.SegmentSize <= 0 {
		c.SegmentSize = SegmentDefaultSize
	}
	if c.LoadFactor <= 0 || c.LoadFactor > 1.0 {
		c.LoadFactor = 0.5
	}
}

// Loader is the write-side component of featcache.
// It creates a shared memory segment, writes data from a DataSource,
// and builds the hash table index. Only one Loader per segment.
type Loader struct {
	segment   *Segment
	hashTable *HashTable
	config    LoaderConfig

	// dataEnd tracks the next free byte in the data region (relative to DataOffset).
	// Accessed atomically by the loader during writes.
	dataEnd int32
}

// NewLoader creates a new Loader. The segment is created if it doesn't exist,
// or opened if it already exists.
func NewLoader(config LoaderConfig) (*Loader, error) {
	config.defaults()

	seg, err := CreateSegment(config.SegmentName, config.SegmentSize)
	if err != nil {
		seg, err = OpenSegment(config.SegmentName)
		if err != nil {
			return nil, err
		}
	}
	return newLoaderWithSegment(config, seg)
}

// newLoaderWithSegment creates a Loader over an existing segment.
// Used internally by NewLoader and by tests to exercise the load path
// without allocating a real shared memory segment.
func newLoaderWithSegment(config LoaderConfig, seg *Segment) (*Loader, error) {
	config.defaults()

	l := &Loader{
		segment: seg,
		config:  config,
	}
	return l, nil
}

// Init initializes the segment header and hash table from an estimated entry count.
// Call this before Load. It computes the optimal hash table size and sets up the layout.
func (l *Loader) Init(expectedEntries int) error {
	segSize := l.segment.Cap()

	// Hash table sizing: capacity = next_pow2(entries * 2) for load factor ~50%
	slotsNeeded := uint32(expectedEntries) * 2
	hashCap := NextPow2(slotsNeeded)
	hashBytes := int(hashCap) * SlotSize

	// Layout:
	// [Header:64B] [HashTable:hashBytes] [DataRegion:remaining]
	hashOffset := HeaderSize
	dataOffset := Align(uint32(hashOffset)+uint32(hashBytes), 8)

	if int(dataOffset) >= segSize {
		return errors.New("segment too small for expected entries")
	}

	// Write header
	hdr := l.segment.Header()
	hdr.Magic = Magic
	hdr.Version = Version
	hdr.Size = uint64(segSize)
	hdr.GenCounter = 0
	hdr.HashCap = hashCap
	hdr.HashOffset = uint32(hashOffset)
	hdr.DataOffset = dataOffset
	hdr.DataEnd = dataOffset // data region starts empty
	hdr.SegmentID = 0

	// Initialize hash table (zero all slots)
	InitHashTable(l.segment.Data(), hashOffset, int(hashCap))

	// Create HashTable handle
	l.hashTable = NewHashTable(l.segment.Data(), hashOffset, int(dataOffset), int(hashCap))
	l.dataEnd = int32(dataOffset)

	return nil
}

// Load reads all entries from the DataSource and writes them to the segment.
// Returns the number of entries loaded.
func (l *Loader) Load(ds DataSource) (int, error) {
	total, err := ds.Open()
	if err != nil {
		return 0, err
	}
	defer ds.Close()

	// If Init wasn't called, initialize with the estimated count
	if l.hashTable == nil {
		if err := l.Init(total); err != nil {
			return 0, err
		}
	}

	count := 0
	for {
		key, value, err := ds.Next()
		if err == ErrEOF {
			break
		}
		if err != nil {
			return count, err
		}
		if len(key) == 0 || len(key) > MaxKeyLen {
			continue // skip invalid entries
		}

		if err := l.put(key, value); err != nil {
			if errors.Is(err, errHashFull) {
				log.Printf("featcache: hash table full after %d entries, consider larger segment", count)
				break
			}
			return count, err
		}
		count++
	}

	// Update generation counter
	l.segment.Header().GenCounter++
	log.Printf("featcache: loaded %d entries into segment %q", count, l.config.SegmentName)
	return count, nil
}

// errHashFull is returned when the hash table cannot accept more entries.
var errHashFull = errors.New("featcache: hash table full")

// put writes a key-value pair into the segment.
func (l *Loader) put(key, value []byte) error {
	data := l.segment.Data()

	// Compute chunk size: [keyLen:4B][key][value]
	chunkSize := 4 + len(key) + len(value)

	// Allocate space in data region via CAS on dataEnd
	dataOff := atomic.AddInt32(&l.dataEnd, int32(chunkSize)) - int32(chunkSize)
	if int(dataOff)+chunkSize > len(data) {
		return errors.New("featcache: data region full")
	}

	absOff := int(dataOff)

	// Write data: [keyLen:4B][key][value]
	binary.LittleEndian.PutUint32(data[absOff:absOff+4], uint32(len(key)))
	copy(data[absOff+4:], key)
	copy(data[absOff+4+len(key):], value)

	// Insert into hash table
	h := HashKey(key)
	relOffset := uint32(absOff) - l.segment.Header().DataOffset
	if !l.hashTable.Insert(h, key, relOffset, uint32(len(value))) {
		return errHashFull
	}

	return nil
}

// Close closes the underlying segment (but does NOT destroy it).
func (l *Loader) Close() error {
	return l.segment.Close()
}

// Destroy closes and unlinks the segment.
func (l *Loader) Destroy() error {
	return l.segment.Destroy()
}

// Segment returns the underlying segment.
func (l *Loader) Segment() *Segment {
	return l.segment
}

// HashTable returns the hash table.
func (l *Loader) HashTable() *HashTable {
	return l.hashTable
}
