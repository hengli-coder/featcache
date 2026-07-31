//go:build !linux

package featcache

func createSegment(name string, size int) (*Segment, error) {
	return nil, ErrNotSupported
}

func openSegment(name string) (*Segment, error) {
	return nil, ErrNotSupported
}

func (s *Segment) close() error {
	return ErrNotSupported
}

func (s *Segment) destroy() error {
	return ErrNotSupported
}
