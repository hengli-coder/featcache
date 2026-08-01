# Architecture Overview

> featcache: load once, share across processes with zero-copy reads, hot-swap ready.

## 1. System goals

**featcache** is a zero-copy runtime data cache for AI inference. It solves one problem:

> Inference processes re-load large static datasets (embeddings, tokenizer vocabularies, feature dictionaries) on every startup — slow startup, wasted memory.

With a traditional setup, N inference processes each load their own copy of 10GB+ of data:

- Minutes of startup latency
- Memory usage of N × data size
- A full reload on every process restart

featcache loads the data **once**, writes it into shared memory, and every process reads the same physical memory via `mmap`.

## 2. Roles

```
┌──────────────────────────────────────────────────────────┐
│                 featcache daemon (featload)              │
│                 = Loader / Artifact Manager              │
│                                                          │
│  ▲ The single writer                                    │
│  ▲ Loads inference artifacts from a DataSource          │
│  ▲ Writes to the shared memory segment, builds the      │
│    hash index                                           │
│  ▲ Serves the control plane over UDS                    │
└──────────────┬───────────────────────────────────────────┘
               │  shm + mmap
               ▼
┌──────────────────────────────────────────────────────────┐
│                  Shared Memory Segment                   │
│                                                          │
│  ┌──────────────────────────────────────────────────┐    │
│  │  Data Plane                                       │    │
│  │  • Header (64B)                                   │    │
│  │  • Hash index: open addressing, O(1) lookup       │    │
│  │  • Data region: compact contiguous key+value      │    │
│  │  • Zero-copy reads, lock-free, syscall-free       │    │
│  └──────────────────────────────────────────────────┘    │
│                                                          │
│  Control Plane → Unix Domain Socket                      │
│  • Segment metadata for client initialization            │
│  • Version-change notification on hot swap (Phase 2)     │
└──────────────┬───────────────────────────────────────────┘
               │  mmap (all processes share the same physical memory)
               ▼
┌──────────────────────────────────────────────────────────┐
│  Inference proc 0   proc 1   proc 2  ...  proc N         │
│                                                          │
│  ▲ Read-only access                                      │
│  ▲ Direct reads from the segment, zero-copy              │
│  ▲ Lookup latency ≈ local map read                       │
│  ▲ Lock-free, syscall-free, no UDS traffic               │
└──────────────────────────────────────────────────────────┘
```

### Key principle: control plane and data plane are separated

| Dimension | Control plane (UDS) | Data plane (shared memory) |
|-----------|---------------------|----------------------------|
| Purpose | Initialization, metadata, version notification | All data reads |
| Frequency | Once per process startup | Every lookup |
| Latency | Microseconds (acceptable) | Nanoseconds |
| Protocol | Custom binary protocol | None (direct memory access) |

## 3. Component responsibilities

| Component | File | Responsibility |
|-----------|------|----------------|
| `Segment` | `segment.go` | Platform-independent segment create/open/close/destroy API |
| `Segment (linux)` | `segment_linux.go` | Linux mmap implementation (`/dev/shm` + `unix.Mmap`) |
| `Segment (other)` | `segment_other.go` | Non-Linux stubs (return `ErrNotSupported`; in-memory close/destroy for tests) |
| `HashTable` | `hashtable.go` | Open-addressed + linear-probing hash table; CAS writes, atomic reads |
| `HashKey` | `hash.go` | `hash/maphash` 64-bit hash |
| `Loader` | `loader.go` | The single writer: reads a DataSource, writes the segment, builds the index |
| `Reader` | `reader.go` | Zero-copy reader: queries the shared-memory hash table directly |
| `CacheServer` | `server.go` | UDS control-plane server (`OpGetInfo` / `OpGetStatus`) |
| `DataSource` | `datasource.go` | DataSource abstraction + built-ins (file / line / map) |
| Protocol codec | `protocol.go` | UDS binary protocol encode/decode |
| `featload` | `cmd/featload/` | Loader daemon entry point |

## 4. Data flow

### 4.1 Load path (write side)

```
Loader starts
  │
  ├─ 1. Create/open the shared memory segment (shm_open + mmap)
  ├─ 2. Init(n): size the hash table from the entry estimate; init Header + table
  ├─ 3. Load(ds) calls ds.Open() → totalEntries
  ├─ 4. Loop over ds.Next():
  │      ├─ write to the data region [keyLen:4B][key][value]
  │      ├─ advance data_end atomically
  │      └─ insert the hash slot (data first, slot marked after)
  ├─ 5. GenCounter++ publishes the new version
  └─ 6. Ready; listen on UDS
```

### 4.2 Read path (query side)

```
Inference process starts
  ├─ 1. Connect to UDS
  ├─ 2. GET_INFO → segment name, size, layout
  ├─ 3. mmap the shared memory segment
  ├─ 4. Close the UDS connection (never used again)
  └─ 5. Query: GET(key) searches the shared-memory hash table directly
```

> **Note on the current implementation**: `Reader.connect` opens the segment directly; the GET_INFO exchange is a planned TODO (see [roadmap](../design/roadmap.md)).

### 4.3 Request lifecycle

```
Reader.Get(key):
  1. hash = HashKey(key)                       // computed locally
  2. idx = hash & (hashCap - 1)                // locate the slot
  3. Probe loop:
       slot.status == empty  → NOT_FOUND
       slot.status == used && slot.hash == hash
         && key matches      → return value   // a view into shared memory
  4. Whole path: no locks, no syscalls, no UDS traffic
```

## 5. Extension points

| Extension point | Interface / location | Notes |
|-----------------|----------------------|-------|
| Data sources | `DataSource` interface | Implement file, database, object store, stream, ... |
| Value format | unrestricted | Values are opaque byte sequences; callers define serialization |
| Protocol OpCodes | `types.go` | `OpWatch`/`OpPin`/`OpPrefetch`/`OpEvict`/`OpList`/`OpReload` reserved for Phases 2–3 |
| Hot swap | Phase 2 | Double buffering + atomic pointer switch |

## 6. Performance considerations

| Metric | Target | Key design support |
|--------|--------|--------------------|
| Startup latency < 100 ms | Any data size | mmap + a single UDS lookup |
| Lookup latency < 100 ns | Single Get | Lock-free atomic reads + hash comparison, no syscalls |
| Memory efficiency ~1.002× | Data size | Compact storage, no internal fragmentation |
| Multi-process savings (N−1)× | N processes | Shared physical memory |

The performance bottleneck is only on the write path (one-time); the read path is pure memory access.

## 7. Security considerations

| Risk | Mitigation |
|------|------------|
| Shared memory segment name collision | Unique segment names + `O_EXCL` creation |
| Segment permissions | 0600 file permissions, same-user access only |
| Out-of-bounds reads | Bounds checks on hash table and read paths |
| Unauthorized UDS access | Filesystem permissions in production; abstract namespace is same-user only |
| Writer crash | Segment stays in `/dev/shm`, reusable after restart (clean up with `Destroy`) |

See [SECURITY.md](../../SECURITY.md) for more.

## 8. Related documents

- [Memory layout](memory-layout.md)
- [Concurrency model](concurrency.md)
- [Control-plane protocol](control-plane.md)
- [Architecture Decision Records](../design/ADRs.md)
- [Roadmap](../design/roadmap.md)
