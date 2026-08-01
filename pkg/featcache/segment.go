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
	"unsafe"
)

// ErrNotSupported is returned on non-Linux platforms.
var ErrNotSupported = errors.New("shared memory is not supported on this platform")

// Segment is a handle to a shared memory segment.
// It can be either a writer (owns the segment) or a reader (opens an existing one).
type Segment struct {
	name string
	data []byte
	cap  int

	// mapped reports whether data was obtained from unix.Mmap (Linux) and
	// must be unmapped on close. In-memory test segments skip munmap.
	mapped bool
}

// CreateSegment creates a new shared memory segment with the given name and size.
// On Linux, the segment is backed by /dev/shm/<name> and mmap'd with MAP_SHARED.
func CreateSegment(name string, size int) (*Segment, error) {
	return createSegment(name, size)
}

// OpenSegment opens an existing shared memory segment by name.
func OpenSegment(name string) (*Segment, error) {
	return openSegment(name)
}

// Close unmaps the shared memory segment. Does NOT unlink the backing file.
func (s *Segment) Close() error {
	return s.close()
}

// Destroy unmaps the segment AND unlinks the backing file.
// Other processes that still have the segment mapped can continue using it;
// new callers must Create again.
func (s *Segment) Destroy() error {
	return s.destroy()
}

// Data returns the mapped byte slice (entire segment).
func (s *Segment) Data() []byte { return s.data }

// Cap returns the total capacity of the segment.
func (s *Segment) Cap() int { return s.cap }

// Name returns the segment name.
func (s *Segment) Name() string { return s.name }

// Header returns a pointer to the segment header at offset 0.
func (s *Segment) Header() *Header {
	return (*Header)(unsafe.Pointer(&s.data[0]))
}

// HashOffset returns the byte offset where the hash table starts.
func (s *Segment) HashOffset() int {
	return int(s.Header().HashOffset)
}

// DataOffset returns the byte offset where the data region starts.
func (s *Segment) DataOffset() int {
	return int(s.Header().DataOffset)
}

// HashCap returns the hash table capacity (number of slots).
func (s *Segment) HashCap() int {
	return int(s.Header().HashCap)
}

// GenCounter returns the current generation counter.
func (s *Segment) GenCounter() uint64 {
	return s.Header().GenCounter
}
