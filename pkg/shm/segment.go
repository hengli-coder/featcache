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

// Package shm provides a minimal, platform-abstracted POSIX shared memory
// segment primitive: create or open a named segment, map it into the
// process address space, and hand back the raw bytes.
//
// It is deliberately generic — it has no knowledge of what's stored inside
// the segment. Higher-level packages (e.g. featcache's on-disk header and
// hash-table format) build on top of the raw []byte returned by Data.
package shm

import "errors"

// ErrNotSupported is returned on non-Linux platforms.
var ErrNotSupported = errors.New("shared memory is not supported on this platform")

// Segment is a handle to a shared memory segment.
// It can be either a writer (owns the segment) or a reader (opens an existing one).
type Segment struct {
	name string
	data []byte
	cap  int

	// mapped reports whether data was obtained from unix.Mmap (Linux) and
	// must be unmapped on close. In-memory segments skip munmap.
	mapped bool

	// backedByFile reports whether this segment owns a backing file that must
	// be unlinked on destroy. Linux segments (Create/Open) are true by
	// default; in-memory segments and borrowed segments (see Borrow) are not.
	backedByFile bool
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

// Borrow marks the segment as not owning its backing file, so a later
// Destroy unmaps the memory but skips unlinking. Use this when a caller
// shares an already-open segment (e.g. a control-plane server sharing the
// loader's segment) without taking ownership of its on-disk lifecycle.
func (s *Segment) Borrow() {
	s.backedByFile = false
}

// NewInMemorySegment creates a Segment backed by a plain Go byte slice
// instead of real shared memory. It is intended for tests that exercise
// the logic built on top of Segment without needing actual cross-process
// mmap semantics.
func NewInMemorySegment(name string, size int) *Segment {
	return &Segment{name: name, data: make([]byte, size), cap: size}
}
