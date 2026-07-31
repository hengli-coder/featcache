//go:build !linux

package featcache

func createSegment(_ string, _ int) (*Segment, error) {
	return nil, ErrNotSupported
}

func openSegment(_ string) (*Segment, error) {
	return nil, ErrNotSupported
}

func (s *Segment) close() error {
	return ErrNotSupported
}

func (s *Segment) destroy() error {
	return ErrNotSupported
}
