package featcache

import "hash/maphash"

var seed = maphash.MakeSeed()

// HashKey returns a 64-bit hash of the given key using maphash.
// The seed is initialized at package load time and consistent within
// a process. For cross-process consistency, the Loader should set
// the seed and share it via the Header (reserved bytes).
func HashKey(key []byte) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	h.Write(key)
	return h.Sum64()
}
