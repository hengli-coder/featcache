//go:build linux

package featcache

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func devShmPath(name string) string {
	return filepath.Join("/dev/shm", name)
}

func createSegment(name string, size int) (*Segment, error) {
	path := devShmPath(name)
	fd, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
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
		name: name,
		data: data,
		cap:  size,
	}, nil
}

func openSegment(name string) (*Segment, error) {
	path := devShmPath(name)
	fd, err := os.OpenFile(path, os.O_RDWR, 0600)
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
		name: name,
		data: data,
		cap:  size,
	}, nil
}

func (s *Segment) close() error {
	if s.data == nil {
		return nil
	}
	err := unix.Munmap(s.data)
	s.data = nil
	return err
}

func (s *Segment) destroy() error {
	_ = s.close()
	return os.Remove(devShmPath(s.name))
}
