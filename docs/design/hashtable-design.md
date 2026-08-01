# HashTable Design

## 1. Problem

Store a key→value index in shared memory, supporting:

- Single writer (Loader) writes; multiple readers (processes/goroutines) read concurrently
- Lock-free read path
- Large capacity (millions to hundreds of millions of entries)

## 2. Goals

- Average lookup O(1)
- Lock-free, syscall-free reads
- At 50% load factor: ~1.5 average probes, 99% of lookups ≤ 5 probes
- 24B/slot; two slots within one cache line

## 3. Non-Goals

- Space reclamation after deletion (Phase 1)
- Dynamic resizing (fixed capacity)
- Concurrent writes (single writer)

## 4. Proposed Design

### 4.1 Algorithm: open addressing + linear probing

```
slot layout (24B):
[hash:8B][offset:4B][vlen:4B][status:4B][pad:4B]

Lookup:
idx = hash & (cap-1)
for i in 0..cap:
  status = atomic_load(slots[idx].status)
  empty → NOT_FOUND
  used && hash matches && key matches → FOUND
  tomb → keep probing
  idx = (idx+1) & mask
```

### 4.2 Concurrency safety

| Operation | Mechanism |
|-----------|-----------|
| Slot write | CAS-claim an empty/tomb slot → write data fields → mark Used |
| Slot read | atomic status load (acquire); data precedes the marker |
| Deletion | atomically replace status with Tomb |

### 4.3 Store the full 64-bit hash

- Reduces key comparisons after false hits (keys are typically long)
- False-hit rate: 1/2^64, negligible

## 5. Alternatives Considered

| Option | Pros | Cons | Conclusion |
|--------|------|------|------------|
| Open addressing (chosen) | cache-friendly, no pointers | deletion needs tombstones | ✅ |
| Chained hashing | simple deletion | pointer overhead, cache-unfriendly | ❌ |
| Cuckoo hashing | worst-case O(1) | complex writes, rehash risk | ❌ |
| Skip list / tree | ordered traversal | complex, slow | ❌ |

## 6. Performance characteristics

At 50% load factor:

| Metric | Value |
|--------|-------|
| Average probes | ≈ 1.5 |
| 99% of lookups | ≤ 5 probes |
| Cost per probe | 1 atomic read + 1 hash comparison |
| Extra cost on hash hit | 1 key comparison |

## 7. Risks

| Risk | Mitigation |
|------|------------|
| Table full (insufficient capacity) | `Insert` returns failure; the Loader logs and stops; docs recommend sizing at 2× the estimate |
| False-hit key comparison | full 64-bit hash stored; false-hit rate 2^-64 |
| Tombstone buildup | no updates in Phase 1; tombstones are reusable |

## 8. Related Code

- `pkg/featcache/hashtable.go`
- `pkg/featcache/types.go` (HashSlot, constants)
- Tests: `pkg/featcache/featcache_test.go`, `pkg/featcache/server_test.go`
