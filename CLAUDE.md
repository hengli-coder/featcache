# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**featcache** is a zero-copy runtime data cache for AI inference. It solves the problem of slow startup caused by loading large static data (embeddings, tokenizer vocabularies, feature dictionaries) across multiple inference processes.

Architecture: single-writer (Loader) + multiple-reader (inference processes).

- The `featload` daemon loads data from a source, writes it to a POSIX shared memory segment, and builds a hash index
- Inference processes mmap the same segment and read directly — zero-copy, no locks, no syscalls on the read path
- Communication over Unix Domain Socket (abstract namespace) for control-plane operations only (initial metadata lookup, version notifications)
- Data-plane reads go directly through shared memory, never through UDS

## Build & Test Commands

```bash
# Build the loader daemon (Linux only)
go build ./cmd/featload

# Run all tests
go test ./pkg/featcache/ -v -count=1

# Run tests with coverage
go test ./pkg/featcache/ -coverprofile=coverage.out -covermode=atomic -count=1
go tool cover -func=coverage.out

# Run benchmarks
go test ./pkg/featcache/ -bench=. -benchmem -count=1

# Run a specific test
go test ./pkg/featcache/ -v -run TestHashTable -count=1
```

## Platform Support

Linux only. The codebase uses build tags:
- `//go:build linux` — real implementation using `golang.org/x/sys/unix` for mmap
- `//go:build !linux` — stubs returning `ErrNotSupported`

Tests use in-memory byte slices to test core logic on non-Linux platforms.

## Memory Layout

```
Offset 0:      Header (64B) — Magic, Version, Size, HashCap, GenCounter, etc.
After header:  Hash Table (open-addressed, linear probing, 24B per slot)
After hash:    Data Region (compact key+value storage, append-only)
```

No slab allocator. Data is stored compactly: `[keyLen:4B][keyBytes][valueBytes]` per entry.

## Concurrency Model

- **Writer (Loader)**: CAS to claim hash slots; atomic store for status. Writes data first, then marks slot as used.
- **Readers (inference processes)**: Atomic load slot status, then read data directly — no locks, no syscalls.
- **Hash table**: Tombstones (`SlotTomb`) preserve probe sequences after deletion.

## Key Components

| File | Purpose |
|------|---------|
| `pkg/featcache/types.go` | Constants, Header, HashSlot, OpCodes |
| `pkg/featcache/hash.go` | `HashKey()` / `HashKeyWithSeed()` — seeded FNV-1a (cross-process) |
| `pkg/shm/segment.go` | Cross-platform `Segment` API wrapper |
| `pkg/shm/segment_linux.go` | Linux mmap/munmap |
| `pkg/shm/segment_other.go` | Non-Linux stubs + in-memory segments for tests |
| `pkg/featcache/hashtable.go` | Open-addressed hash table with atomic CAS |
| `pkg/featcache/loader.go` | Batch loader — reads from DataSource, writes to segment |
| `pkg/featcache/reader.go` | Zero-copy reader — GET_INFO over UDS, then direct shared memory |
| `pkg/featcache/datasource.go` | DataSource interface (file, line, map) |
| `pkg/featcache/protocol.go` | UDS binary protocol for control plane |
| `pkg/featcache/server.go` | CacheServer: `OpGetInfo` / `OpGetStatus` |

## Development Notes

- Go version: 1.25 (as specified in go.mod)
- Only external dependency: `golang.org/x/sys` for Unix syscalls
- Tests allocate large byte slices (64MB+) for realistic benchmarking — may need to increase timeout for CI
- The `dataPtr()` helper uses `unsafe.Pointer` to cast slice data for struct overlays