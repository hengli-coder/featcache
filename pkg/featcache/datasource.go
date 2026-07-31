package featcache

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// ErrEOF signals that the data source is exhausted.
var ErrEOF = errors.New("featcache: end of data source")

// DataSource defines the interface for loading key-value pairs into the cache.
// Implementations should return ErrEOF when all data has been read.
//
// Typical usage:
//
//	ds, _ := NewFileDataSource("/path/to/data")
//	total, _ := ds.Open()
//	loader.Load(ds, total)
//	ds.Close()
type DataSource interface {
	// Open prepares the data source and returns the total number of entries
	// (or an estimate). Used to pre-size the hash table.
	Open() (totalEntries int, err error)

	// Next reads the next key-value pair. Returns ErrEOF when done.
	Next() (key []byte, value []byte, err error)

	// Close releases resources held by the data source.
	Close() error
}

// --- FileDataSource ---
//
// Reads key-value pairs from a binary file. Each entry is encoded as:
//
//	[keyLen: uint32 LE][key: keyLen bytes][valueLen: uint32 LE][value: valueLen bytes]
//
// The file must use this exact format. Total entries are computed from file size.

// FileDataSource reads key-value pairs from a binary file.
type FileDataSource struct {
	path string
	f    *os.File
	br   *bufio.Reader
}

// NewFileDataSource creates a FileDataSource for the given path.
func NewFileDataSource(path string) *FileDataSource {
	return &FileDataSource{path: path}
}

// Open opens the file and computes the total number of entries.
func (ds *FileDataSource) Open() (int, error) {
	f, err := os.Open(ds.path)
	if err != nil {
		return 0, err
	}
	ds.f = f
	ds.br = bufio.NewReader(f)

	// Compute total entries from file size (best-effort estimate).
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return 0, err
	}
	// Each entry is at least 8 bytes (two uint32 headers).
	estimate := int(info.Size() / 8)
	return estimate, nil
}

// Next reads the next key-value pair.
func (ds *FileDataSource) Next() ([]byte, []byte, error) {
	// Read keyLen
	var keyLen uint32
	if err := binary.Read(ds.br, binary.LittleEndian, &keyLen); err != nil {
		if err == io.EOF {
			return nil, nil, ErrEOF
		}
		return nil, nil, err
	}

	// Read key
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(ds.br, key); err != nil {
		return nil, nil, err
	}

	// Read valueLen
	var valLen uint32
	if err := binary.Read(ds.br, binary.LittleEndian, &valLen); err != nil {
		return nil, nil, err
	}

	// Read value
	val := make([]byte, valLen)
	if _, err := io.ReadFull(ds.br, val); err != nil {
		return nil, nil, err
	}

	return key, val, nil
}

// Close closes the underlying file.
func (ds *FileDataSource) Close() error {
	if ds.f != nil {
		return ds.f.Close()
	}
	return nil
}

// --- LineDataSource ---
//
// Reads key-value pairs from a text file where each line is "key\tvalue\n".
// Useful for testing and quick data loading.

// LineDataSource reads key-value pairs from a tab-separated text file.
type LineDataSource struct {
	path string
	f    *os.File
	sc   *bufio.Scanner
	n    int
}

// NewLineDataSource creates a LineDataSource for the given path.
func NewLineDataSource(path string) *LineDataSource {
	return &LineDataSource{path: path}
}

// Open opens the file and counts lines for an estimate.
func (ds *LineDataSource) Open() (int, error) {
	f, err := os.Open(ds.path)
	if err != nil {
		return 0, err
	}
	ds.f = f
	ds.sc = bufio.NewScanner(f)
	ds.n = 0
	return 0, nil // unknown count until we scan
}

// Next reads the next key-value pair (tab-separated line).
func (ds *LineDataSource) Next() ([]byte, []byte, error) {
	if !ds.sc.Scan() {
		if err := ds.sc.Err(); err != nil {
			return nil, nil, err
		}
		return nil, nil, ErrEOF
	}
	line := ds.sc.Bytes()
	ds.n++

	// Split on first tab
	for i, b := range line {
		if b == '\t' {
			key := make([]byte, i)
			copy(key, line[:i])
			val := make([]byte, len(line)-i-1)
			copy(val, line[i+1:])
			return key, val, nil
		}
	}
	// No tab found — key is the whole line, value is empty
	key := make([]byte, len(line))
	copy(key, line)
	return key, nil, nil
}

// Close closes the underlying file.
func (ds *LineDataSource) Close() error {
	if ds.f != nil {
		return ds.f.Close()
	}
	return nil
}
