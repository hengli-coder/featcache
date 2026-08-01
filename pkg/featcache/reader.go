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
	"errors"
	"net"
	"sync"
	"time"
)

// Reader is the read-side component of featcache.
// It opens an existing shared memory segment and provides zero-copy
// key-value lookups. No locks, no syscalls, no UDS round-trips on the
// hot path.
//
// Multiple goroutines can safely call Get concurrently.
type Reader struct {
	segment   *Segment
	hashTable *HashTable

	// Control plane (UDS) — used only during initialization
	mu     sync.Mutex
	conn   net.Conn
	closed bool
}

// NewReader opens an existing segment and connects to the loader via UDS
// to obtain the segment layout. After initialization, all queries go
// directly through shared memory.
func NewReader(segmentName string, udsAddr string) (*Reader, error) {
	r := &Reader{}

	// Connect to loader and get segment metadata
	if udsAddr != "" {
		if err := r.connect(udsAddr, segmentName); err != nil {
			return nil, err
		}
	} else {
		// No UDS — open segment directly (assumes layout is known)
		seg, err := OpenSegment(segmentName)
		if err != nil {
			return nil, err
		}
		r.segment = seg
		r.initHashTable()
	}

	return r, nil
}

// NewReaderFromSegment creates a Reader from an already-opened Segment.
// Useful for testing or when you manage segment lifecycle yourself.
func NewReaderFromSegment(seg *Segment) (*Reader, error) {
	r := &Reader{
		segment: seg,
	}
	r.initHashTable()
	return r, nil
}

func (r *Reader) initHashTable() {
	hdr := r.segment.Header()
	r.hashTable = NewHashTable(
		r.segment.Data(),
		int(hdr.HashOffset),
		int(hdr.DataOffset),
		int(hdr.HashCap),
	)
}

func (r *Reader) connect(udsAddr, segmentName string) error {
	conn, err := net.DialTimeout("unix", udsAddr, 5*time.Second)
	if err != nil {
		return err
	}
	r.conn = conn

	// TODO: Send GET_INFO request and receive segment metadata
	// For now, just open the segment directly
	seg, err := OpenSegment(segmentName)
	if err != nil {
		conn.Close()
		return err
	}
	r.segment = seg
	r.initHashTable()
	return nil
}

// Get looks up a key and returns the value directly from shared memory.
// The returned byte slice is a view into mmap'd memory — callers must NOT
// modify it.
//
// Returns (value, true) on hit, (nil, false) on miss.
func (r *Reader) Get(key []byte) ([]byte, bool) {
	h := HashKey(key)
	return r.hashTable.Get(h, key)
}

// GetBatch looks up multiple keys. Returns values and existence flags in the same order.
func (r *Reader) GetBatch(keys [][]byte) (values [][]byte, results []bool) {
	values = make([][]byte, len(keys))
	results = make([]bool, len(keys))
	for i, key := range keys {
		values[i], results[i] = r.Get(key)
	}
	return values, results
}

// GenCounter returns the current generation counter from the segment header.
func (r *Reader) GenCounter() uint64 {
	return r.segment.GenCounter()
}

// Close closes the UDS connection and unmaps the segment.
func (r *Reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true

	if r.conn != nil {
		r.conn.Close()
	}
	return r.segment.Close()
}

// Segment returns the underlying segment.
func (r *Reader) Segment() *Segment {
	return r.segment
}

// ErrNotConnected is returned when a Reader operation fails due to missing connection.
var ErrNotConnected = errors.New("featcache: not connected")
