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
	"encoding/binary"
	"fmt"
	"testing"
)

// BenchmarkGet measures single-key lookup latency through a Reader backed by
// an in-memory segment (platform-independent). This exercises the full
// hash-lookup path: hash, slot probe, key match, slice.
func BenchmarkGet(b *testing.B) {
	const n = 100_000

	// build via Loader for a realistic index
	loader := buildBenchLoader(b, n)
	r, err := NewReaderFromSegment(loader.Segment())
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()

	keys := makePostKeys(n)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		key := keys[i%len(keys)]
		if _, ok := r.Get(key); !ok {
			b.Fatal("unexpected miss")
		}
	}
}

// BenchmarkGetHitRatio8 simulates the realistic 50%-loaded lookup hit rate.
func BenchmarkGetKnownKey(b *testing.B) {
	loader := buildBenchLoader(b, 1000)
	r, err := NewReaderFromSegment(loader.Segment())
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()
	key := []byte("key0001")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := r.Get(key); !ok {
			b.Fatal("miss")
		}
	}
}

// BenchmarkGetBatch measures batch lookup throughput.
func BenchmarkGetBatch(b *testing.B) {
	loader := buildBenchLoader(b, 10_000)
	r, err := NewReaderFromSegment(loader.Segment())
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()

	keys := makePostKeys(10_000)
	batch := keys[:64]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, res := r.GetBatch(batch); !res[0] {
			b.Fatal("batch miss")
		}
	}
}

// BenchmarkHashKey isolates purely the hash function cost.
func BenchmarkHashKey(b *testing.B) {
	key := []byte("benchmark-key-for-featcache")
	for i := 0; i < b.N; i++ {
		HashKey(key)
	}
}

// --- helpers ---

func buildBenchLoader(b *testing.B, n int) *Loader {
	seg := newBenchSegment(n)
	l, err := newLoaderWithSegment(LoaderConfig{SegmentName: "bench"}, seg)
	if err != nil {
		b.Fatal(err)
	}
	if err := l.Init(n); err != nil {
		b.Fatal(err)
	}
	entries := make(map[string][]byte, n)
	for i := 0; i < n; i++ {
		entries[fmt.Sprintf("key%04d", i)] = makePostValue(i)
	}
	if _, err := l.Load(NewMapDataSource(entries)); err != nil {
		b.Fatal(err)
	}
	return l
}

func newBenchSegment(n int) *Segment {
	// Loader.Init uses hashCap = NextPow2(2n) slots (rounded to power of 2),
	// so reserve that exact hash table size. Keys are "key%04d" (6-8 bytes),
	// values 32 bytes, so each entry is [keyLen:4][key][val:32] ~ 44 bytes.
	hashCap := NextPow2(uint32(n) * 2)
	hashBytes := int(hashCap) * SlotSize
	dataOffset := Align(uint32(64+hashBytes), 8)
	entryBytes := 4 + 8 + 32
	sz := int(dataOffset) + n*entryBytes + 8192 /*slack*/
	return &Segment{name: "bench", data: make([]byte, sz), cap: sz}
}

func makePostKeys(n int) [][]byte {
	keys := make([][]byte, n)
	for i := 0; i < n; i++ {
		keys[i] = []byte(fmt.Sprintf("key%04d", i))
	}
	return keys
}

func makePostValue(i int) []byte {
	// Simulate a small embedding: ~32-byte payload in a fixed pattern.
	v := make([]byte, 32)
	binary.BigEndian.PutUint32(v, uint32(i))
	for j := 4; j < len(v); j++ {
		v[j] = byte(j)
	}
	return v
}
