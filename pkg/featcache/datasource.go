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
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// ErrEOF signals that the data source is exhausted.
var ErrEOF = errors.New("featcache: end of data source")

// --- MapDataSource ---

// MapDataSource reads key-value pairs from an in-memory map.
// Useful for tests, demos, and embedding small datasets at runtime.
type MapDataSource struct {
	entries map[string][]byte
	keys    []string
	idx     int
}

// NewMapDataSource creates a MapDataSource from a map.
func NewMapDataSource(entries map[string][]byte) *MapDataSource {
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	return &MapDataSource{entries: entries, keys: keys}
}

// Open returns the total number of entries.
func (ds *MapDataSource) Open() (int, error) {
	return len(ds.entries), nil
}

// Next reads the next key-value pair.
func (ds *MapDataSource) Next() (key []byte, value []byte, err error) {
	if ds.idx >= len(ds.keys) {
		return nil, nil, ErrEOF
	}
	k := ds.keys[ds.idx]
	ds.idx++
	return []byte(k), ds.entries[k], nil
}

// Close is a no-op for an in-memory data source.
func (ds *MapDataSource) Close() error { return nil }

// DataSource defines the interface for loading key-value pairs into the cache.
// Implementations should return ErrEOF when all data has been read.
//
// Typical usage:
//
//	ds := NewFileDataSource("/path/to/data")
//	loader.Init(1000) // pre-size the hash table from an entry estimate
//	loader.Load(ds)   // Load calls ds.Open() internally
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
func (ds *FileDataSource) Next() (key []byte, value []byte, err error) {
	// Read keyLen
	var keyLen uint32
	if err := binary.Read(ds.br, binary.LittleEndian, &keyLen); err != nil {
		if err == io.EOF {
			return nil, nil, ErrEOF
		}
		return nil, nil, err
	}

	// Read key
	key = make([]byte, keyLen)
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
func (ds *LineDataSource) Next() (key []byte, value []byte, err error) {
	if !ds.sc.Scan() {
		if err := ds.sc.Err(); err != nil {
			return nil, nil, err
		}
		return nil, nil, ErrEOF
	}
	line := ds.sc.Bytes()
	ds.n++

	// Split on first tab.
	for i, b := range line {
		if b != '\t' {
			continue
		}
		key = make([]byte, i)
		copy(key, line[:i])
		value = make([]byte, len(line)-i-1)
		copy(value, line[i+1:])
		return key, value, nil
	}
	// No tab found — key is the whole line, value is empty.
	key = make([]byte, len(line))
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
