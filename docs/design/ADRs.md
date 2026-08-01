# Architecture Decision Records (ADRs)

This file records featcache's key architecture decisions. Each ADR follows the "Context → Decision → Rationale → Consequences" structure.

## Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [ADR-1](#adr-1-why-not-a-slab-allocator) | Compact storage vs. slab allocator | Accepted | 2026-07 |
| [ADR-2](#adr-2-why-do-slots-store-the-full-64-bit-hash-in-24b) | 24B slots store the full hash | Accepted | 2026-07 |
| [ADR-3](#adr-3-why-do-clients-query-the-hash-table-locally-instead-of-over-uds) | Clients query the shared-memory hash table locally | Accepted | 2026-07 |
| [ADR-4](#adr-4-why-no-gc-in-phase-1) | No space reclamation in Phase 1 | Accepted | 2026-07 |
| [ADR-5](#adr-5-why-posix-shm-instead-of-file-mmap) | POSIX shared memory | Accepted | 2026-07 |
| [ADR-6](#adr-6-why-hashmaphash-for-hashing) | `hash/maphash` for hashing | Accepted | 2026-07 |

---

## ADR-1: Why not a slab allocator?

**Status**: Accepted · **Date**: 2026-07-19

**Decision**: compact storage with append-only writes.

**Rationale**:

- Phase 1 is "write once, read-only", so no dynamic allocation is needed
- Compact storage eliminates internal fragmentation — significant savings at 10GB+ scale (30%+)
- Hot swap replaces data by creating a new segment + atomic switch; the old segment is reclaimed whole
- Simpler implementation, lower complexity

**Consequences**:

- Data updates/deletes (Phase 2) go through version switching, not in-place modification
- Append-only means space cannot be freed during the write phase

---

## ADR-2: Why do slots store the full 64-bit hash in 24B?

**Status**: Accepted · **Date**: 2026-07-19

**Decision**: 24B slots store the full 64-bit hash.

**Rationale**:

- Full-hash comparison greatly reduces key comparisons (keys are typically long, e.g. `user:embedding:12345`)
- Two slots occupy 48B — one cache line — prefetch-friendly
- The extra 8B/slot is negligible: 8MB per million entries vs. 10GB+ of data

**Consequences**:

- Hash table memory overhead ≈ 2.4% (at 50% load factor, 24B slot per 48B of data)
- False-hit rate is negligible; lookup performance is stable

---

## ADR-3: Why do clients query the hash table locally instead of over UDS?

**Status**: Accepted · **Date**: 2026-07-19

**Decision**: clients query the shared-memory hash table locally; UDS is used only for initialization.

**Rationale**:

- Eliminates UDS round-trips (microseconds → nanoseconds)
- Eliminates the server bottleneck (N concurrent clients add no server load)
- Zero-copy data plane: no serialization/deserialization overhead
- Simpler architecture: the read path is stateless with no connection management

**Consequences**:

- Clients must be able to open the shared memory segment (same-host deployment prerequisite)
- Hot swap needs an extra version-notification mechanism (Phase 2, `OpWatch`)

---

## ADR-4: Why no GC in Phase 1?

**Status**: Accepted · **Date**: 2026-07-19

**Decision**: no space reclamation in Phase 1.

**Rationale**:

- Phase 1 is "load once, unchanged at runtime"; no DELETE/UPDATE
- Phase 2 hot swap replaces data via "new segment + atomic switch"
- In-place GC needs reference counting or mark-and-sweep — high complexity and risk

**Consequences**:

- Deletion only supports logical deletion (tombstones); slots stay reusable
- Space reclamation relies on the Phase 2 version-switch mechanism

---

## ADR-5: Why POSIX shared memory instead of file mmap?

**Status**: Accepted · **Date**: 2026-07-19

**Decision**: use `/dev/shm` + `mmap(MAP_SHARED)`.

**Rationale**:

- POSIX shm is designed for inter-process sharing; mapped on tmpfs (RAM-backed)
- Avoids real disk I/O; performance matches pure memory
- `/dev/shm` is mounted by default — no extra system configuration
- File mmap requires file lifecycle management and complex sync semantics

**Consequences**:

- Linux only (the project's target platform)
- Shared memory is limited by `/dev/shm` capacity (default 50% of RAM, configurable)

---

## ADR-6: Why `hash/maphash` for hashing?

**Status**: Accepted · **Date**: 2026-07-19

**Decision**: use the standard library `hash/maphash` (a strong AES-instruction-based hash).

**Rationale**:

- Standard library implementation, zero external dependencies
- High performance (hardware-accelerated) and collision-resistant
- No need for a third-party hash library

**Consequences**:

- The hash seed is process-local (`maphash.MakeSeed()`), so **hashes are inconsistent across processes**!
- Impact: hashes written by the Loader and hashes computed by Readers may differ

**Current status and risk**: in `hash.go`, the seed is a package-level global initialized independently in each process. **This is a known cross-process consistency defect** that needs fixing:

- Option A (recommended): the Loader generates the seed and shares it via the Header's Reserved bytes
- Option B: use a seedless hash (e.g. FNV-1a) that is deterministic across processes
- See [Issue tracking & roadmap](roadmap.md)
