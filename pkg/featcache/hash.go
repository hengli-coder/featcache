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
	"crypto/rand"
	"encoding/binary"
)

// seedSlot is where the 64-bit hash seed is persisted in the header Reserved
// bytes (8 bytes suffice for a uint64).
const seedSlot = 0 // Reserved[0:8] — where the hash seed is persisted

// fnvOffset64 and fnvPrime64 are the standard FNV-1a constants.
const (
	fnvOffset64 = 14695981039346656037
	fnvPrime64  = 1099511628211
)

// HashKey returns a 64-bit hash of key using the package-level process seed.
// It is only internally consistent within a single process; cross-process
// consumers (loader/reader in different OS processes) must hash with the
// seed persisted in the segment header instead — see HashKeyWithSeed,
// headerHashSeed, and setHeaderHashSeed.
func HashKey(key []byte) uint64 {
	return HashKeyWithSeed(key, defaultSeed())
}

// HashKeyWithSeed returns a 64-bit FNV-1a hash of key, seeded with seed.
//
// Deliberately NOT hash/maphash: maphash.Seed mixes in a per-process random
// AES key inside the runtime (see runtime.memhash), so two processes with
// the *same* Seed value still compute different hashes — the seed alone does
// not make maphash reproducible across processes. FNV-1a here is a plain,
// portable byte-at-a-time hash with no hidden process-local state, so the
// same (seed, key) always yields the same hash everywhere, which is required
// for the loader (writer) and readers (other OS processes) to agree on slot
// placement in the shared hash table.
func HashKeyWithSeed(key []byte, seed uint64) uint64 {
	h := uint64(fnvOffset64) ^ seed
	for _, b := range key {
		h ^= uint64(b)
		h *= fnvPrime64
	}
	return h
}

// headerHashSeed recovers the hash seed persisted in the header Reserved
// bytes. If the header carries no seed yet (all-zero), a fresh process-local
// seed is returned instead, matching the loader's own fallback so hashing
// stays internally consistent before a seed is written.
func headerHashSeed(hdr *Header) uint64 {
	seed := binary.LittleEndian.Uint64(hdr.Reserved[seedSlot : seedSlot+8])
	if seed == 0 {
		return defaultSeed()
	}
	return seed
}

// setHeaderHashSeed writes a fresh random hash seed into the header Reserved
// bytes so all processes sharing the segment agree on key hashing. It returns
// the seed it wrote so callers can hash consistently in the same turn.
func setHeaderHashSeed(hdr *Header) uint64 {
	seed := randSeed()
	binary.LittleEndian.PutUint64(hdr.Reserved[seedSlot:seedSlot+8], seed)
	return seed
}

// pkgSeed is the process-local seed used by the in-process HashKey API.
// Within a single process it is stable, so a HashKey(insert) and
// HashKey(get) in the same binary agree. Cross-process consumers must use
// the header-persisted seed (headerHashSeed) instead.
var pkgSeed = randSeed()

// defaultSeed returns the default process seed.
func defaultSeed() uint64 {
	return pkgSeed
}

// randSeed returns a cryptographically random, non-zero uint64. 0 is
// reserved to mean "no seed persisted yet" (see headerHashSeed).
func randSeed() uint64 {
	var buf [8]byte
	for {
		if _, err := rand.Read(buf[:]); err != nil {
			panic("featcache: crypto/rand unavailable: " + err.Error())
		}
		if seed := binary.LittleEndian.Uint64(buf[:]); seed != 0 {
			return seed
		}
	}
}
