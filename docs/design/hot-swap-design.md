# Hot-Swap Design (Phase 2)

> Status: Proposed · Target version: v1.0.0 (see [roadmap](roadmap.md))

## 1. Problem

Phase 1 data is "written once, unchanged at runtime". In production, embedding tables and feature dictionaries need periodic updates (e.g. daily training produces new models), and restarting inference processes is expensive.

## 2. Goals

- Replace data at runtime without interrupting service
- Queries are briefly eventually-consistent (may read old data at the switch instant)
- No memory leaks (old segments are eventually reclaimed)

## 3. Non-Goals

- In-place updates/deletes (keep append-only + tombstones)
- Cross-machine consistency
- Transactional updates (some readers old, some new is acceptable)

## 4. Proposed Design: double-buffered version switching

```
1. Loader creates a new segment and writes the new version
   ┌──────────────────┐      ┌──────────────────┐
   │ Segment v1       │      │ Segment v2       │
   │ GenCounter = 100 │      │ GenCounter = 101 │  ← new data
   └──────────────────┘      └──────────────────┘
         ↑ inference procs read        ↑ write complete

2. Loader broadcasts the version change over UDS (OpWatch)

3. Each Reader, on notification:
   a. mmap the new segment
   b. atomically replace the internal pointer (in-flight queries keep using the old segment)
   c. keep the old segment (until in-flight queries finish)

4. After all Readers confirm the switch, the Loader reclaims the old segment
```

### 4.1 Protocol extension

```
OpWatch = 0x03  // listen for version-change notifications
```

- The client keeps listening after connecting and triggers a switch when `GenCounter` changes
- Keep the connection (replaces the current "disconnect after init" pattern)

### 4.2 Reader side

```go
type Reader struct {
    // atomic pointer: the currently active segment
    current atomic.Pointer[Segment]
    // old segment references (kept after switching, drained by in-flight queries)
    oldSegments []*Segment
}
```

### 4.3 Incremental updates (optional optimization)

- Data-source diff detection: only update changed keys
- Or full rebuild (simple, reliable): create a new segment and write all data

## 5. Alternatives Considered

| Option | Pros | Cons | Conclusion |
|--------|------|------|------------|
| Double buffering + atomic switch (chosen) | simple, lock-free, no pauses | double memory peak | ✅ |
| In-place update + version number | no memory peak | complex concurrency, tombstone buildup | ❌ |
| Reference counting + in-place GC | space efficient | high complexity | ❌ |

## 6. Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Double memory peak | memory pressure during large updates | documented; segmented loading support |
| Delayed old-segment reclamation | memory held temporarily | reference counting + timer fallback |
| Inconsistency at the switch instant | queries read old data | eventual-consistency design, documented explicitly |
| New segment write failure | version not published | keep the old segment serving; log the error |

## 7. Test Plan

- Unit: atomic pointer switching, reference counting
- Integration: Loader publishes v2 → Reader switches → new data readable
- Stress: concurrent queries during the switch must not crash

## 8. Implementation sequence

Do these in order; each step should be independently testable:

1. **Long-lived control-plane connection** — `handleConn` currently reads one request and closes. Watch needs a persistent connection (or a dedicated watch socket).
2. **`OpWatch`** — client stays connected; server pushes a response when `GenCounter` changes (or on subscribe, send the current generation first).
3. **Loader double-buffer** — create segment v2, `Load` into it, then publish. Keep serving v1 until watchers have switched.
4. **Reader atomic switch** — `atomic.Pointer[*shm.Segment]`; in-flight `Get` uses the pointer it loaded at start of the call.
5. **Reclamation** — refcount in-flight readers + a timer fallback so a stuck watcher cannot pin v1 forever.
6. **Full rebuild first** — skip incremental diffs until the switch path is production-proven.

## 9. Future Improvements

- Diff updates (copy only changed data incrementally)
- Multi-version retention (gray rollback)
