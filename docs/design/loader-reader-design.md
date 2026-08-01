# Loader / Reader Design

## 1. Problem

Need a read/write API pair:

- **Loader**: the single writer, batch-loads from a DataSource into shared memory
- **Reader**: inference processes, zero-copy reads

## 2. Goals

- Loader: batched, capacity-estimable, failure-recoverable
- Reader: < 100ns lookups, lock-free, multi-goroutine safe
- Reader and writer share the same memory-layout description

## 3. Non-Goals

- Updates/deletes (Phase 1)
- Cross-process permission management
- Data validation (value format is opaque)

## 4. Proposed Design

### 4.1 Loader

```go
type Loader struct {
    segment   *Segment
    hashTable *HashTable
    config    LoaderConfig
    dataEnd   int32   // advanced atomically
}

// Usage flow
loader := NewLoader(LoaderConfig{SegmentName, SegmentSize, LoadFactor})
loader.Init(expectedEntries)   // compute layout, init Header + hash table
count, err := loader.Load(ds)  // batch-write from a DataSource
loader.Close() / loader.Destroy()
```

**Write flow** (`put`):

1. `atomic.AddInt32(&dataEnd, chunkSize)` allocates space
2. Write `[keyLen:4B][key][value]`
3. Compute `HashKey(key)`, insert into the hash table (data ready first, slot marked after)

**Capacity protection**:

- `Init` validates that the segment size ≥ header + hash table
- `put` validates remaining data-region space and returns an error when full
- Invalid keys (empty, overlong) are skipped and logged

### 4.2 Reader

```go
type Reader struct {
    segment   *Segment
    hashTable *HashTable
    mu    sync.Mutex  // protects Close state only
    conn  net.Conn    // UDS, initialization only
}

reader := NewReader(segmentName, udsAddr) // or NewReaderFromSegment(seg)
value, ok := reader.Get(key)              // zero-copy, lock-free
values, results := reader.GetBatch(keys)  // batched
reader.GenCounter()                       // version number
reader.Close()
```

**Read path**: `Get` only does `HashKey` + a hash table lookup — no locks, no syscalls.

### 4.3 Initialization flow (production path)

```
NewReader(name, udsAddr)
  ├─ 1. Connect to UDS (5s timeout)
  ├─ 2. GET_INFO → segment metadata (TODO: current implementation opens the segment directly)
  ├─ 3. OpenSegment + mmap
  └─ 4. Build the HashTable handle
```

## 5. Alternatives Considered

| Option | Pros | Cons | Conclusion |
|--------|------|------|------------|
| Loader/Reader split (chosen) | clear responsibilities, write/read permission separation | two APIs | ✅ |
| Single Segment API | simple | mixes write/read, error-prone | ❌ |

## 6. Risks

| Risk | Mitigation |
|------|------------|
| Reader returns a shared-memory slice that gets modified | documented prohibition; callers must copy |
| DataSource fails mid-way | returns loaded count + error; retryable |
| Insufficient segment capacity | double-checked in Init/put with clear errors |
| Cross-process hash inconsistency | see [ADR-6](ADRs.md#adr-6-why-hashmaphash-for-hashing), to be fixed |

## 7. Related Code

- `pkg/featcache/loader.go`
- `pkg/featcache/reader.go`
- Tests: `pkg/featcache/loader_test.go`
