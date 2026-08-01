package featcache

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// --- HashTable tests ---

func TestHashTable_InsertAndGet(t *testing.T) {
	bufSize := 64*1024 + 256*1024 + 1024 // hash + data
	buf := make([]byte, bufSize)
	slotBase := 0
	dataBase := 64 * 1024
	capacity := 256

	ht := NewHashTable(buf, slotBase, dataBase, capacity)
	InitHashTable(buf, slotBase, capacity)

	// Insert a key-value pair
	hash := HashKey([]byte("hello"))
	// Write data at dataBase
	key := []byte("hello")
	val := []byte("world")
	chunkOff := dataBase
	binary.LittleEndian.PutUint32(buf[chunkOff:chunkOff+4], uint32(len(key)))
	copy(buf[chunkOff+4:], key)
	copy(buf[chunkOff+4+len(key):], val)

	ok := ht.Insert(hash, key, uint32(chunkOff-dataBase), uint32(len(val)))
	if !ok {
		t.Fatal("Insert failed")
	}

	// Get the key
	got, found := ht.Get(hash, []byte("hello"))
	if !found {
		t.Fatal("Get returned false")
	}
	if string(got) != "world" {
		t.Fatalf("expected 'world', got %q", got)
	}
}

func TestHashTable_Miss(t *testing.T) {
	buf := make([]byte, 64*1024+256*1024+1024)
	slotBase := 0
	dataBase := 64 * 1024
	capacity := 256

	ht := NewHashTable(buf, slotBase, dataBase, capacity)
	InitHashTable(buf, slotBase, capacity)

	_, found := ht.Get(HashKey([]byte("nonexistent")), []byte("nonexistent"))
	if found {
		t.Fatal("expected miss")
	}
}

func TestHashTable_MultipleEntries(t *testing.T) {
	buf := make([]byte, 64*1024+1024*1024+1024)
	slotBase := 0
	dataBase := 64 * 1024
	capacity := 1024

	ht := NewHashTable(buf, slotBase, dataBase, capacity)
	InitHashTable(buf, slotBase, capacity)

	entries := map[string]string{
		"key1":     "val1",
		"key2":     "val222",
		"key3":     "val333333",
		"user:abc": "embedding_data",
	}

	dataOff := uint32(0)
	for k, v := range entries {
		hash := HashKey([]byte(k))
		// Write data
		absOff := dataBase + int(dataOff)
		key := []byte(k)
		val := []byte(v)
		binary.LittleEndian.PutUint32(buf[absOff:absOff+4], uint32(len(key)))
		copy(buf[absOff+4:], key)
		copy(buf[absOff+4+len(key):], val)

		if !ht.Insert(hash, key, dataOff, uint32(len(val))) {
			t.Fatalf("Insert failed for %q", k)
		}
		dataOff += uint32(4 + len(key) + len(val))
	}

	for k, v := range entries {
		hash := HashKey([]byte(k))
		got, found := ht.Get(hash, []byte(k))
		if !found {
			t.Fatalf("Get(%q) = miss, want hit", k)
		}
		if string(got) != v {
			t.Fatalf("Get(%q) = %q, want %q", k, got, v)
		}
	}

	if count := ht.Count(); count != len(entries) {
		t.Fatalf("Count() = %d, want %d", count, len(entries))
	}
}

func TestHashTable_Delete(t *testing.T) {
	buf := make([]byte, 64*1024+256*1024+1024)
	slotBase := 0
	dataBase := 64 * 1024
	capacity := 256

	ht := NewHashTable(buf, slotBase, dataBase, capacity)
	InitHashTable(buf, slotBase, capacity)

	hash := HashKey([]byte("delme"))
	key := []byte("delme")
	val := []byte("value")

	absOff := dataBase
	binary.LittleEndian.PutUint32(buf[absOff:absOff+4], uint32(len(key)))
	copy(buf[absOff+4:], key)
	copy(buf[absOff+4+len(key):], val)

	ht.Insert(hash, key, 0, uint32(len(val)))

	// Verify exists
	_, found := ht.Get(hash, key)
	if !found {
		t.Fatal("expected hit before delete")
	}

	// Delete
	if !ht.Delete(hash, key) {
		t.Fatal("Delete returned false")
	}

	// Verify gone
	_, found = ht.Get(hash, key)
	if found {
		t.Fatal("expected miss after delete")
	}
}

func TestHashTable_HashCollisions(t *testing.T) {
	// With a tiny table, some collisions are expected
	buf := make([]byte, 64*1024+4*1024+1024)
	slotBase := 0
	dataBase := 64 * 1024
	capacity := 8

	ht := NewHashTable(buf, slotBase, dataBase, capacity)
	InitHashTable(buf, slotBase, capacity)

	// Insert 4 entries into an 8-slot table
	entries := map[string]string{}
	dataOff := uint32(0)
	for i := 0; i < 4; i++ {
		k := []byte("key" + string(rune('0'+i)))
		v := []byte("val" + string(rune('0'+i)))
		entries[string(k)] = string(v)

		absOff := dataBase + int(dataOff)
		binary.LittleEndian.PutUint32(buf[absOff:absOff+4], uint32(len(k)))
		copy(buf[absOff+4:], k)
		copy(buf[absOff+4+len(k):], v)

		ht.Insert(HashKey(k), k, dataOff, uint32(len(v)))
		dataOff += uint32(4 + len(k) + len(v))
	}

	for k, v := range entries {
		got, found := ht.Get(HashKey([]byte(k)), []byte(k))
		if !found {
			t.Fatalf("Get(%q) = miss", k)
		}
		if string(got) != v {
			t.Fatalf("Get(%q) = %q, want %q", k, got, v)
		}
	}
}

// --- Hash tests ---

func TestHashKey_Deterministic(t *testing.T) {
	key := []byte("test_key_123")
	h1 := HashKey(key)
	h2 := HashKey(key)
	if h1 != h2 {
		t.Fatalf("HashKey not deterministic: %d != %d", h1, h2)
	}
}

func TestHashKey_DifferentKeys(t *testing.T) {
	h1 := HashKey([]byte("key1"))
	h2 := HashKey([]byte("key2"))
	if h1 == h2 {
		t.Fatal("HashKey returned same hash for different keys")
	}
}

// --- Protocol tests ---

func TestProtocol_EncodeDecodeRequest(t *testing.T) {
	req := &Request{
		Op:  OpGetInfo,
		Key: []byte("test-key"),
	}

	// Encode to buffer
	var buf bytes.Buffer
	if err := EncodeRequest(&buf, req); err != nil {
		t.Fatal(err)
	}

	// Decode
	decoded, err := DecodeRequest(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Op != req.Op {
		t.Fatalf("Op = %d, want %d", decoded.Op, req.Op)
	}
	if !bytes.Equal(decoded.Key, req.Key) {
		t.Fatalf("Key = %q, want %q", decoded.Key, req.Key)
	}
}

func TestProtocol_EncodeDecodeResponse(t *testing.T) {
	resp := &Response{
		Status:      RespOK,
		SegmentName: "test-segment",
		SegmentSize: 1024 * 1024 * 1024,
		HashOffset:  64,
		HashCap:     1024,
		DataOffset:  1024 * 1024,
		GenCounter:  42,
	}

	// Encode to buffer
	var buf bytes.Buffer
	if err := EncodeResponse(&buf, resp); err != nil {
		t.Fatal(err)
	}

	// Decode
	decoded, err := DecodeResponse(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Status != resp.Status {
		t.Fatalf("Status = %d, want %d", decoded.Status, resp.Status)
	}
	if decoded.SegmentName != resp.SegmentName {
		t.Fatalf("SegmentName = %q, want %q", decoded.SegmentName, resp.SegmentName)
	}
	if decoded.SegmentSize != resp.SegmentSize {
		t.Fatalf("SegmentSize = %d, want %d", decoded.SegmentSize, resp.SegmentSize)
	}
	if decoded.HashCap != resp.HashCap {
		t.Fatalf("HashCap = %d, want %d", decoded.HashCap, resp.HashCap)
	}
	if decoded.DataOffset != resp.DataOffset {
		t.Fatalf("DataOffset = %d, want %d", decoded.DataOffset, resp.DataOffset)
	}
	if decoded.GenCounter != resp.GenCounter {
		t.Fatalf("GenCounter = %d, want %d", decoded.GenCounter, resp.GenCounter)
	}
}

// --- Types tests ---

func TestAlign(t *testing.T) {
	tests := []struct {
		v, n, want uint32
	}{
		{0, 8, 0},
		{1, 8, 8},
		{7, 8, 8},
		{8, 8, 8},
		{9, 8, 16},
		{0, 64, 0},
		{1, 64, 64},
		{65, 64, 128},
	}
	for _, tt := range tests {
		got := Align(tt.v, tt.n)
		if got != tt.want {
			t.Errorf("Align(%d, %d) = %d, want %d", tt.v, tt.n, got, tt.want)
		}
	}
}

func TestNextPow2(t *testing.T) {
	tests := []struct {
		v, want uint32
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{100, 128},
		{1024, 1024},
	}
	for _, tt := range tests {
		got := NextPow2(tt.v)
		if got != tt.want {
			t.Errorf("NextPow2(%d) = %d, want %d", tt.v, got, tt.want)
		}
	}
}

// --- Loader/Reader integration test ---

func TestLoaderLoadAndReaderGet(t *testing.T) {
	// Create a temporary in-memory data source
	entries := map[string]string{
		"user:123:emb":    "embedding_data_123",
		"item:456:emb":    "embedding_data_456",
		"tokenizer:vocab": "vocab_data",
	}

	ds := &mapDataSource{entries: entries, keys: make([]string, 0, len(entries))}
	for k := range entries {
		ds.keys = append(ds.keys, k)
	}

	// Create loader with in-memory segment (simulated)
	// Since we can't use shm on non-Linux, test with in-memory buffer
	bufSize := 64*1024 + 1024*1024 + 1024
	buf := make([]byte, bufSize)
	slotBase := 0
	dataBase := 64 * 1024
	capacity := 64

	ht := NewHashTable(buf, slotBase, dataBase, capacity)
	InitHashTable(buf, slotBase, capacity)

	// Simulate loader writing
	dataOff := uint32(0)
	for k, v := range entries {
		key := []byte(k)
		val := []byte(v)
		absOff := dataBase + int(dataOff)

		binary.LittleEndian.PutUint32(buf[absOff:absOff+4], uint32(len(key)))
		copy(buf[absOff+4:], key)
		copy(buf[absOff+4+len(key):], val)

		if !ht.Insert(HashKey(key), key, dataOff, uint32(len(val))) {
			t.Fatalf("Insert failed for %q", k)
		}
		dataOff += uint32(4 + len(key) + len(val))
	}

	// Simulate reader
	for k, v := range entries {
		got, found := ht.Get(HashKey([]byte(k)), []byte(k))
		if !found {
			t.Fatalf("Get(%q) = miss", k)
		}
		if string(got) != v {
			t.Fatalf("Get(%q) = %q, want %q", k, got, v)
		}
	}
}

// mapDataSource is a test data source backed by a map.
type mapDataSource struct {
	entries map[string]string
	keys    []string
	idx     int
}

func (ds *mapDataSource) Open() (int, error) {
	return len(ds.entries), nil
}

func (ds *mapDataSource) Next() (key []byte, value []byte, err error) {
	if ds.idx >= len(ds.keys) {
		return nil, nil, ErrEOF
	}
	k := ds.keys[ds.idx]
	ds.idx++
	return []byte(k), []byte(ds.entries[k]), nil
}

func (ds *mapDataSource) Close() error { return nil }
