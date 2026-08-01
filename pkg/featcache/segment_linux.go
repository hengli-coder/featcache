//go:build linux

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
	"os"

	"golang.org/x/sys/unix"
)

func devShmPath(name string) string {
	return "/dev/shm/" + name
}

func createSegment(name string, size int) (*Segment, error) {
	path := devShmPath(name)
	fd, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	if err := fd.Truncate(int64(size)); err != nil {
		os.Remove(path)
		return nil, err
	}

	data, err := unix.Mmap(int(fd.Fd()), 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		os.Remove(path)
		return nil, err
	}

	return &Segment{
		name:   name,
		data:   data,
		cap:    size,
		mapped: true,
	}, nil
}

func openSegment(name string) (*Segment, error) {
	path := devShmPath(name)
	fd, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	info, err := fd.Stat()
	if err != nil {
		return nil, err
	}
	size := int(info.Size())

	data, err := unix.Mmap(int(fd.Fd()), 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}

	return &Segment{
		name:   name,
		data:   data,
		cap:    size,
		mapped: true,
	}, nil
}

func (s *Segment) close() error {
	if s.data == nil {
		return nil
	}
	// Only unmap memory obtained from unix.Mmap. In-memory test segments
	// (make([]byte, ...)) are not registered with the syscall mapper and
	// Munmap would return EINVAL ("invalid argument").
	if s.mapped {
		s.mapped = false
		if err := unix.Munmap(s.data); err != nil {
			return err
		}
	}
	s.data = nil
	return nil
}

func (s *Segment) destroy() error {
	_ = s.close()
	return os.Remove(devShmPath(s.name))
}
