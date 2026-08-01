# Concurrency Model

featcache uses a **single-writer / multiple-reader** concurrency model: one Loader writes, N Reader processes read concurrently. The read path is completely lock-free and syscall-free.

## 1. Model overview

```
┌──────────────┐        ┌─────────────────────┐
│  Loader      │  writes│  Shared memory seg  │
│  (single     │ ─────► │  [Header]           │
│   writer)    │        │  [Hash Slots]       │
│              │        │  [Data Region]      │
└──────────────┘        └──────────┬──────────┘
                                   │ read-only
                     ┌─────────────┼─────────────┐
                     ▼             ▼             ▼
              ┌────────────┐ ┌────────────┐ ┌────────────┐
              │ Reader 1   │ │ Reader 2   │ │ Reader N   │
              │ lock-free  │ │ lock-free  │ │ lock-free  │
              └────────────┘ └────────────┘ └────────────┘
```

## 2. The writer (Loader)

### 2.1 Data region writes

```go
// 1. Compute the chunk size and advance data_end atomically
dataOff := atomic.AddInt32(&l.dataEnd, int32(chunkSize)) - int32(chunkSize)

// 2. Write the data: [keyLen:4B][key][value]
binary.LittleEndian.PutUint32(data[absOff:absOff+4], uint32(len(key)))
copy(data[absOff+4:], key)
copy(data[absOff+4+len(key):], value)

// 3. Insert into the hash table (data is already in place)
ht.Insert(hash, key, relOffset, uint32(len(value)))
```

**Write ordering guarantee**: the data is fully written to the data region *before* the hash slot is marked `SlotUsed`. This is the core of safe reading.

### 2.2 Hash table slot writes

```go
// CAS-claim an empty slot (avoids multi-writer conflicts)
if !atomic.CompareAndSwapUint32(statusPtr, status, SlotUsed) {
    // claim failed, probe the next slot
    continue
}
// claimed → write hash/offset/vlen
```

## 3. The reader (Reader)

### 3.1 Read flow

```go
// Atomic load of the slot status (acquire semantics)
status := atomic.LoadUint32((*uint32)(unsafe.Pointer(&ht.data[off+16])))

// Status check
switch slot.Status {
case SlotEmpty:
    return nil, false        // empty slot: not found
case SlotUsed:
    // ready → read hash/offset/vlen → compare key → return value
case SlotTomb:
    // tombstone: keep probing
}
```

**Key points**:

- Readers only do **atomic loads**, never writes
- A reader either sees an empty slot (data not ready) or complete written data
- **There is no half-written state** (the writer writes data first, then marks the slot)

### 3.2 Why lock-free?

1. **Single writer**: only one process writes, no write-write exclusion needed
2. **Atomic publication**: the slot marker is an atomic store; readers do atomic loads
3. **Immutable data**: data does not change after the initial load (Phase 1); readers never conflict
4. **No reference counting**: segment lifetime is managed by the Loader

## 4. Atomicity guarantees

### 4.1 Memory ordering

| Operation | Semantics | Guarantee |
|-----------|-----------|-----------|
| Write to the data region | plain store | happens before the slot marker (program order) |
| Mark the slot | atomic store (release) | readers see complete data |
| Read the slot | atomic load (acquire) | sees the complete write |

Go's `atomic` package provides the appropriate hardware barriers on supported platforms, safe across architectures (not just x86).

### 4.2 Reader concurrency

- Multiple goroutines can safely call `Reader.Get` concurrently (no shared mutable state)
- Multiple processes reading shared memory concurrently never conflict (read-only)

## 5. Deletion and tombstones

Phase 1 only supports logical deletion (`SlotTomb`):

```go
// Delete: atomically replace the status with Tombstone
atomic.StoreUint32(statusPtr, SlotTomb)
```

- Tombstones preserve the probe sequence, keeping linear probing contiguous
- Tombstone slots can be reused by a later `Insert` (CAS claim)

## 6. Known limitations and Phase 2 improvements

| Limitation | Notes | Phase 2 plan |
|------------|-------|--------------|
| No in-place updates | written once in Phase 1 | double-buffered version switch |
| No space reclamation | deletions leave tombstones | replace the whole segment |
| Single writer | no multi-Loader support | keep single writer (simpler) |

## 7. Related code

| Location | Content |
|----------|---------|
| `pkg/featcache/hashtable.go` | CAS writes, atomic reads, tombstones |
| `pkg/featcache/loader.go` | atomic data-region advancement |
| `pkg/featcache/reader.go` | lock-free reads |
