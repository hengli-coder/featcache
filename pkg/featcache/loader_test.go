package featcache

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Segment helpers (in-memory, platform-independent) ---

// newTestSegment builds an in-memory segment with the given size.
// Exercises the full Loader → Reader path without Linux shared memory.
func newTestSegment(t *testing.T, size int) *Segment {
	t.Helper()
	if size <= 0 {
		size = 64*1024 + 1024*1024
	}
	return &Segment{
		name: "test-segment",
		data: make([]byte, size),
		cap:  size,
	}
}

// --- Loader tests ---

func TestLoaderConfigDefaults(t *testing.T) {
	var c LoaderConfig
	c.defaults()
	if c.SegmentSize != SegmentDefaultSize {
		t.Fatalf("SegmentSize = %d, want default %d", c.SegmentSize, SegmentDefaultSize)
	}
	if c.LoadFactor != 0.5 {
		t.Fatalf("LoadFactor = %f, want 0.5", c.LoadFactor)
	}

	c = LoaderConfig{SegmentSize: 123, LoadFactor: 0.9}
	c.defaults()
	if c.SegmentSize != 123 {
		t.Fatalf("SegmentSize = %d, want 123", c.SegmentSize)
	}
	if c.LoadFactor != 0.9 {
		t.Fatalf("LoadFactor = %f, want 0.9", c.LoadFactor)
	}

	// Invalid load factor falls back to 0.5.
	c = LoaderConfig{LoadFactor: 1.5}
	c.defaults()
	if c.LoadFactor != 0.5 {
		t.Fatalf("LoadFactor = %f, want 0.5 fallback", c.LoadFactor)
	}
}

func TestLoaderInitAndLayout(t *testing.T) {
	seg := newTestSegment(t, 64*1024+1024*1024)
	l, err := newLoaderWithSegment(LoaderConfig{SegmentName: "t"}, seg)
	if err != nil {
		t.Fatal(err)
	}

	if err := l.Init(10); err != nil {
		t.Fatal(err)
	}

	hdr := seg.Header()
	if hdr.Magic != Magic {
		t.Fatalf("Magic = %x, want %x", hdr.Magic, Magic)
	}
	if hdr.HashCap < 32 { // next_pow2(10*2)=32
		t.Fatalf("HashCap = %d, want >= 32", hdr.HashCap)
	}
	if hdr.HashOffset != HeaderSize {
		t.Fatalf("HashOffset = %d, want %d", hdr.HashOffset, HeaderSize)
	}
	if hdr.DataOffset < hdr.HashOffset+hdr.HashCap*SlotSize {
		t.Fatalf("DataOffset = %d, hash table ends at %d", hdr.DataOffset, hdr.HashOffset+hdr.HashCap*SlotSize)
	}
	if hdr.DataEnd != hdr.DataOffset {
		t.Fatalf("DataEnd = %d, want %d", hdr.DataEnd, hdr.DataOffset)
	}
	if l.HashTable() == nil {
		t.Fatal("hashTable not initialized after Init")
	}
	if l.Segment() != seg {
		t.Fatal("Segment() mismatch")
	}
}

func TestLoaderInitTooSmall(t *testing.T) {
	seg := newTestSegment(t, 64*1024) // too small for a large hash table
	l, err := newLoaderWithSegment(LoaderConfig{}, seg)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Init(1 << 20); err == nil {
		t.Fatal("expected error for segment too small")
	}
}

func TestLoaderLoadWithMapSource(t *testing.T) {
	seg := newTestSegment(t, 0)
	l, err := newLoaderWithSegment(LoaderConfig{SegmentName: "t"}, seg)
	if err != nil {
		t.Fatal(err)
	}

	entries := NewMapDataSource(map[string][]byte{
		"key1": []byte("val1"),
		"key2": []byte("val2"),
	})
	count, err := l.Load(entries)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("loaded %d entries, want 2", count)
	}

	// Verify data is readable via a Reader.
	r, err := NewReaderFromSegment(seg)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	for k, v := range map[string]string{"key1": "val1", "key2": "val2"} {
		got, ok := r.Get([]byte(k))
		if !ok {
			t.Fatalf("Get(%q) = miss, want hit", k)
		}
		if string(got) != v {
			t.Fatalf("Get(%q) = %q, want %q", k, got, v)
		}
	}

	if _, ok := r.Get([]byte("missing")); ok {
		t.Fatal("Get(missing) = hit, want miss")
	}

	// GenCounter must be bumped after load.
	if seg.GenCounter() != 1 {
		t.Fatalf("GenCounter = %d, want 1", seg.GenCounter())
	}
}

func TestLoaderLoadSkipsInvalidKeys(t *testing.T) {
	seg := newTestSegment(t, 0)
	l, err := newLoaderWithSegment(LoaderConfig{SegmentName: "t"}, seg)
	if err != nil {
		t.Fatal(err)
	}

	ds := &mapDataSource{
		entries: map[string]string{
			"valid": "data",
		},
		keys: []string{"valid"},
	}
	ds.keys = append(ds.keys, "") // empty key should be skipped

	count, err := l.Load(ds)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("loaded %d entries, want 1 (invalid key skipped)", count)
	}
}

func TestLoaderLoadDataSourceError(t *testing.T) {
	seg := newTestSegment(t, 0)
	l, err := newLoaderWithSegment(LoaderConfig{SegmentName: "t"}, seg)
	if err != nil {
		t.Fatal(err)
	}

	ds := &errDataSource{}
	if _, err := l.Load(ds); err == nil {
		t.Fatal("expected error from data source Open")
	}
}

// errDataSource returns an error on Open.
type errDataSource struct{}

func (ds *errDataSource) Open() (int, error) { return 0, errors.New("open failed") }
func (ds *errDataSource) Next() (key []byte, value []byte, err error) {
	return nil, nil, ErrEOF
}
func (ds *errDataSource) Close() error { return nil }

// failAfterDataSource fails after N entries.
type failAfterDataSource struct {
	entries map[string][]byte
	keys    []string
	n       int
	err     error
}

func (ds *failAfterDataSource) Open() (int, error) { return len(ds.entries), nil }
func (ds *failAfterDataSource) Next() (key []byte, value []byte, err error) {
	if ds.n >= len(ds.keys) {
		return nil, nil, ErrEOF
	}
	if ds.n == 1 {
		return nil, nil, ds.err
	}
	k := ds.keys[ds.n]
	ds.n++
	return []byte(k), ds.entries[k], nil
}
func (ds *failAfterDataSource) Close() error { return nil }

func TestLoaderLoadPartialFailure(t *testing.T) {
	seg := newTestSegment(t, 0)
	l, err := newLoaderWithSegment(LoaderConfig{SegmentName: "t"}, seg)
	if err != nil {
		t.Fatal(err)
	}

	ds := &failAfterDataSource{
		entries: map[string][]byte{"a": []byte("1")},
		keys:    []string{"a", "b"},
		err:     errors.New("mid-stream failure"),
	}
	count, err := l.Load(ds)
	if err == nil {
		t.Fatal("expected mid-stream error")
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (first entry stored before failure)", count)
	}
}

func TestLoaderCloseAndDestroy(t *testing.T) {
	seg := newTestSegment(t, 0)
	l, err := newLoaderWithSegment(LoaderConfig{SegmentName: "t"}, seg)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	// In-memory segment Close is a no-op; Destroy returns nil (no backing file).
	if err := l.Destroy(); err != nil {
		t.Fatal(err)
	}
}

// --- FileDataSource tests ---

func TestFileDataSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")

	// Write two entries: [keyLen][key][valLen][val].
	var buf bytes.Buffer
	for _, kv := range [][2]string{{"key1", "val1"}, {"key2", "val222"}} {
		binary.Write(&buf, binary.LittleEndian, uint32(len(kv[0])))
		buf.WriteString(kv[0])
		binary.Write(&buf, binary.LittleEndian, uint32(len(kv[1])))
		buf.WriteString(kv[1])
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	ds := NewFileDataSource(path)
	total, err := ds.Open()
	if err != nil {
		t.Fatal(err)
	}
	if total < 2 {
		t.Fatalf("estimate = %d, want >= 2", total)
	}

	var got []string
	for {
		key, val, err := ds.Next()
		if err == ErrEOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, string(key)+"="+string(val))
	}
	if len(got) != 2 {
		t.Fatalf("read %d entries, want 2", len(got))
	}
	if got[0] != "key1=val1" || got[1] != "key2=val222" {
		t.Fatalf("unexpected entries: %v", got)
	}

	if err := ds.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFileDataSourceOpenMissing(t *testing.T) {
	ds := NewFileDataSource(filepath.Join(t.TempDir(), "nope.bin"))
	if _, err := ds.Open(); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFileDataSourceTruncated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trunc.bin")
	// Write a keyLen claiming 100 bytes but no key follows.
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(100))
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	ds := NewFileDataSource(path)
	if _, err := ds.Open(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ds.Next(); err == nil || err == ErrEOF {
		t.Fatalf("expected read error, got %v", err)
	}
}

// --- LineDataSource tests ---

func writeLines(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lines.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLineDataSource(t *testing.T) {
	path := writeLines(t, "key1\tval1", "key2\tval222", "lonelykey")

	ds := NewLineDataSource(path)
	if _, err := ds.Open(); err != nil {
		t.Fatal(err)
	}

	var got []string
	for {
		key, val, err := ds.Next()
		if err == ErrEOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, string(key)+"="+string(val))
	}
	if len(got) != 3 {
		t.Fatalf("read %d lines, want 3", len(got))
	}
	if got[0] != "key1=val1" || got[1] != "key2=val222" || got[2] != "lonelykey=" {
		t.Fatalf("unexpected entries: %v", got)
	}

	if err := ds.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLineDataSourceOpenMissing(t *testing.T) {
	ds := NewLineDataSource(filepath.Join(t.TempDir(), "nope.txt"))
	if _, err := ds.Open(); err == nil {
		t.Fatal("expected error for missing file")
	}
}

// --- MapDataSource tests ---

func TestMapDataSource(t *testing.T) {
	ds := NewMapDataSource(map[string][]byte{
		"a": []byte("1"),
		"b": []byte("2"),
	})
	total, err := ds.Open()
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}

	var n int
	for {
		_, _, err := ds.Next()
		if err == ErrEOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		n++
	}
	if n != 2 {
		t.Fatalf("read %d entries, want 2", n)
	}
	if err := ds.Close(); err != nil {
		t.Fatal(err)
	}
}

// --- Reader tests ---

func TestReaderGetBatch(t *testing.T) {
	seg := newTestSegment(t, 0)
	l, err := newLoaderWithSegment(LoaderConfig{}, seg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = l.Load(NewMapDataSource(map[string][]byte{
		"a": []byte("1"),
		"b": []byte("2"),
	}))
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewReaderFromSegment(seg)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	values, results := r.GetBatch([][]byte{[]byte("a"), []byte("zzz"), []byte("b")})
	if !results[0] || string(values[0]) != "1" {
		t.Fatalf("batch[0] = %q,%v", values[0], results[0])
	}
	if results[1] {
		t.Fatalf("batch[1] = %q,%v, want miss", values[1], results[1])
	}
	if !results[2] || string(values[2]) != "2" {
		t.Fatalf("batch[2] = %q,%v", values[2], results[2])
	}

	if r.GenCounter() != 1 {
		t.Fatalf("GenCounter = %d, want 1", r.GenCounter())
	}
	if r.Segment() != seg {
		t.Fatal("Segment() mismatch")
	}
}

func TestReaderCloseIdempotent(t *testing.T) {
	r := &Reader{
		segment: &Segment{name: "t", data: make([]byte, 64*1024+1024*1024), cap: 64*1024 + 1024*1024},
	}
	r.initHashTable()
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close must be a no-op, got %v", err)
	}
}

// --- Protocol edge cases ---

func TestProtocolRequestMaxKeyTruncation(t *testing.T) {
	var buf bytes.Buffer
	req := &Request{Op: OpGetInfo, Key: bytes.Repeat([]byte("x"), 70000)}
	if err := EncodeRequest(&buf, req); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeRequest(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Key) != 65535 {
		t.Fatalf("key truncated to %d bytes, want 65535", len(decoded.Key))
	}
}

func TestProtocolDecodeShortRequest(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(byte(OpGetInfo))
	if _, err := DecodeRequest(&buf); err != io.ErrUnexpectedEOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestProtocolDecodeShortResponse(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(byte(RespOK))
	if _, err := DecodeResponse(&buf); err != io.ErrUnexpectedEOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestProtocolResponseLongNameTruncation(t *testing.T) {
	var buf bytes.Buffer
	resp := &Response{
		Status:      RespOK,
		SegmentName: strings.Repeat("n", 100),
		SegmentSize: 42,
	}
	if err := EncodeResponse(&buf, resp); err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeResponse(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.SegmentName) > 64 {
		t.Fatalf("SegmentName length %d, want <= 64", len(decoded.SegmentName))
	}
}

func TestProtocolDecodeMissingKeyLen(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{byte(OpGetInfo), 0x00})
	if _, err := DecodeRequest(&buf); err == nil {
		t.Fatal("expected error for missing key bytes")
	}
}
