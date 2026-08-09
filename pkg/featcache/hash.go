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
	"unsafe"

	"hash/maphash"
)

// seedSlot is where the maphash.Seed word is persisted in the header Reserved
// bytes. maphash.Seed wraps a single uint64 (Seed.s), so 8 bytes suffice.
const (
	seedSlot = 0 // Reserved[0:8] — where the maphash.Seed word is persisted
)

// HashKey returns a 64-bit hash of key using the package-level seed. The seed
// is the one written into the segment header by the loader at Init and shared
// across processes (see setHeaderHashSeed / headerHashSeed); the same Seed
// value makes loader and reader agree on slot placement.
func HashKey(key []byte) uint64 {
	return HashKeyWithSeed(key, defaultSeed())
}

// HashKeyWithSeed returns a 64-bit hash of key using seed.
func HashKeyWithSeed(key []byte, seed maphash.Seed) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	h.Write(key)
	return h.Sum64()
}

// headerHashSeed recovers the maphash.Seed persisted in the header Reserved
// bytes. If the header carries no seed yet (all-zero), a fresh process-local
// seed is returned instead, matching the loader's own fallback so hashing stays
// internally consistent before a seed is written.
func headerHashSeed(hdr *Header) maphash.Seed {
	var s maphash.Seed
	cell := hdr.Reserved[seedSlot : seedSlot+8]
	if binary.LittleEndian.Uint64(cell) == 0 {
		return defaultSeed()
	}
	// Seed wraps one uint64; read it back verbatim.
	setSeedWord(&s, binary.LittleEndian.Uint64(cell))
	return s
}

// setHeaderHashSeed writes a fresh random maphash.Seed into the header Reserved
// bytes so all processes sharing the segment agree on key hashing. It returns
// the seed it wrote so callers can hash consistently in the same turn.
func setHeaderHashSeed(hdr *Header) maphash.Seed {
	s := maphash.MakeSeed()
	binary.LittleEndian.PutUint64(hdr.Reserved[seedSlot:seedSlot+8], seedWord(s))
	return s
}

// seedWord exposes the single uint64 stored inside a maphash.Seed.
func seedWord(s maphash.Seed) uint64 {
	return *(*uint64)(unsafe.Pointer(&s))
}

// setSeedWord overwrites the uint64 stored inside a maphash.Seed.
func setSeedWord(s *maphash.Seed, v uint64) {
	*(*uint64)(unsafe.Pointer(s)) = v
}

// defaultSeed is the process-local seed used by the in-process HashKey API.
// Within a single process it is stable (one MakeSeed per package load), so
// a HashKey(insert) and HashKey(get) in the same binary agree. Cross-process
// consumers must use the header-persisted seed (headerHashSeed) instead.
var pkgSeed = maphash.MakeSeed()

// defaultSeed returns the default process seed.
func defaultSeed() maphash.Seed {
	return pkgSeed
}
