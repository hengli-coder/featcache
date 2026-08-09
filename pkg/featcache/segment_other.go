//go:build !linux

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

// testSegments is a platform-independent registry enabling tests to share
// in-memory segments by name across the UDS boundary, mirroring what real
// /dev/shm provides on Linux. It is only exercised by tests.
var testSegments = map[string]*Segment{}

// registerTestSegment makes an in-memory segment openable by name, so a
// Reader (or Loader) can re-open it by name the same way it would shm_open
// on Linux.
func registerTestSegment(name string, seg *Segment) {
	if name != "" {
		testSegments[name] = seg
	}
}

func createSegment(_ string, _ int) (*Segment, error) {
	return nil, ErrNotSupported
}

func openSegment(name string) (*Segment, error) {
	if s, ok := testSegments[name]; ok {
		return &Segment{name: s.name, data: s.data, cap: s.cap, mapped: s.mapped}, nil
	}
	return nil, ErrNotSupported
}

func (s *Segment) close() error {
	if s.data == nil {
		return nil
	}
	// In-memory segment: no-op, just clear the data.
	_ = s.mapped
	s.data = nil
	return nil
}

func (s *Segment) destroy() error {
	_ = s.close()
	delete(testSegments, s.name)
	return nil
}
