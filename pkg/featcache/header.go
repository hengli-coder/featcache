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
	"unsafe"

	"github.com/hengli-coder/featcache/pkg/shm"
)

// headerOf returns a pointer to the featcache Header stored at the start of
// seg's mapped memory. shm.Segment itself knows nothing about this layout —
// it just hands back raw bytes — so featcache owns the unsafe overlay onto
// its own on-disk format.
func headerOf(seg *shm.Segment) *Header {
	return (*Header)(unsafe.Pointer(&seg.Data()[0]))
}
