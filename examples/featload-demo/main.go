//go:build linux

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

// Command featload-demo demonstrates the featcache zero-copy workflow:
// a Loader writes data into a shared memory segment, then a Reader reads it
// back directly from shared memory.
package main

import (
	"fmt"
	"log"

	featcache "github.com/hengli-coder/featcache/pkg/featcache"
)

func main() {
	const (
		segmentName = "featcache-demo"
		segmentSize = 64 * 1024 * 1024 // 64 MB
	)

	// ─── Loader side (normally the featload daemon) ───────────────────
	loader, err := featcache.NewLoader(featcache.LoaderConfig{
		SegmentName: segmentName,
		SegmentSize: segmentSize,
	})
	if err != nil {
		log.Fatalf("create loader: %v", err)
	}

	if err := loader.Init(2); err != nil {
		log.Fatalf("init loader: %v", err)
	}

	entries := featcache.NewMapDataSource(map[string][]byte{
		"user:123:emb":    []byte("embedding_data_123"),
		"tokenizer:vocab": []byte("vocab_data"),
		"item:456:emb":    []byte("embedding_data_456"),
		"feature:dict":    []byte("sparse_feature_dictionary"),
	})
	count, err := loader.Load(entries)
	if err != nil {
		log.Fatalf("load: %v", err)
	}
	fmt.Printf("✓ loaded %d entries into segment %q\n", count, segmentName)

	// ─── Reader side (normally an inference process) ──────────────────
	// NewReaderFromSegment is used here for a single-process demo.
	// In production, inference processes call featcache.NewReader(name, udsAddr)
	// to discover and mmap the shared segment.
	reader, err := featcache.NewReaderFromSegment(loader.Segment())
	if err != nil {
		log.Fatalf("create reader: %v", err)
	}

	// ─── Zero-copy lookups ────────────────────────────────────────────
	val, ok := reader.Get([]byte("user:123:emb"))
	if !ok {
		log.Fatal("key not found")
	}
	fmt.Printf("✓ GET user:123:emb → %q (len=%d)\n", val, len(val))

	// Batch lookup
	values, results := reader.GetBatch([][]byte{
		[]byte("tokenizer:vocab"),
		[]byte("missing:key"),
	})
	fmt.Printf("✓ batch: found=%v value=%q\n", results[0], values[0])
	fmt.Printf("✓ batch: found=%v (expected miss)\n", results[1])

	fmt.Println("\nDone! All operations completed successfully.")

	// Explicit cleanup: the reader must be closed before the segment is
	// destroyed so the in-memory segment's data is still valid.
	if err := reader.Close(); err != nil {
		log.Printf("close reader: %v", err)
	}
	if err := loader.Destroy(); err != nil {
		log.Printf("destroy loader: %v", err)
	}
}
