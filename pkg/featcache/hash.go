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
