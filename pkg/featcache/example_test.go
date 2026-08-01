package featcache

import (
	"encoding/binary"
	"fmt"
)

// ExampleReader_Get demonstrates the Reader zero-copy lookup API.
// It builds an in-memory segment and reads from it, mirroring the
// single-writer / multiple-reader model without requiring Linux shared memory.
func ExampleReader_Get() {
	const bufSize = 64*1024 + 1024*1024
	buf := make([]byte, bufSize)
	const dataBase = 64 * 1024

	// Writer side: build the hash table and store data.
	ht := NewHashTable(buf, 0, dataBase, 256)
	InitHashTable(buf, 0, 256)

	key := []byte("user:embedding:v1")
	value := []byte("0123456789abcdef") // 128-bit embedding, for illustration

	binary.LittleEndian.PutUint32(buf[dataBase:dataBase+4], uint32(len(key)))
	copy(buf[dataBase+4:], key)
	copy(buf[dataBase+4+len(key):], value)
	ht.Insert(HashKey(key), key, 0, uint32(len(value)))

	// Reader side: direct lookup in shared memory — zero-copy.
	got, ok := ht.Get(HashKey(key), key)
	fmt.Printf("found=%v value=%q\n", ok, got)
	// Output: found=true value="0123456789abcdef"
}
