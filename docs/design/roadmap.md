# Roadmap

featcache's feature evolution roadmap, organized by phase. Each phase delivers a shippable milestone.

## Phase 1: Core (complete, v0.3.0)

**Goal**: a usable, testable, deployable core caching system.

| Feature | Status |
|---------|--------|
| Segment management (create/open/close/destroy) | ✅ |
| Header read/write | ✅ |
| HashTable (full 64-bit hash, open addressing) | ✅ |
| Loader (batch loading) | ✅ |
| Reader (zero-copy reads) | ✅ |
| UDS control-plane protocol (`OpGetInfo` / `OpGetStatus`) | ✅ |
| `GET_STATUS` reports `ServerState` (Idle/Loading/Ready/Updating) | ✅ |
| DataSource abstraction + built-ins (file / line / map) | ✅ |
| Tests + benchmarks | ✅ |

**Fixed**:

- [x] Cross-process hash seed consistency (see [ADR-6](ADRs.md#adr-6-why-hashmaphash-for-hashing)) — seed persisted in the Header, hashed with FNV-1a instead of `hash/maphash`
- [x] `Reader.connect` sends GET_INFO and validates the loader-reported segment name
- [x] `featload` daemon accepts a `-source` data source flag

**Do not freeze the public API as v1.0 yet.** Phase 2 hot swap will change `Reader` (atomic current-segment pointer, long-lived `OpWatch` connections). **v0.3.0** is the Phase 1 close-out tag.

## Phase 2: Hot swap (next)

**Goal**: replace data at runtime without interrupting service.

| Feature | Status |
|---------|--------|
| Long-lived UDS + `OpWatch` protocol | next |
| Double-buffered version switching | planned |
| Old-segment reference counting and reclamation | planned |
| Incremental updates + diff detection | later (full rebuild first) |

Suggested implementation order (see [hot-swap-design.md](hot-swap-design.md)):

1. Keep the control-plane connection open and implement `OpWatch` (notify on `GenCounter` change)
2. Loader writes a new segment, then publishes the version
3. Reader switches via `atomic.Pointer`; in-flight `Get` calls keep the old mapping
4. Reclaim the old segment after readers drain (refcount + timeout fallback)
5. Ship full-rebuild hot swap before attempting incremental diffs

## Phase 3: Enhancements

**Goal**: production-grade features.

| Feature | Priority |
|---------|----------|
| Multi-tier storage (GPU → RAM → NVMe → Object Store) | High |
| Persistence (data on disk, restart recovery) | High |
| Metrics (Prometheus: hit rate, latency, memory) | Medium |
| Distributed (multi-host sharing, consistent-hash routing) | Low |
| Compressed storage (feature-vector compression) | Low |

## Version planning (SemVer)

| Version | Contents | Notes |
|---------|----------|-------|
| v0.1.0 | First usable Phase 1 release | Tagged 2026-07-19; changelog dated 2026-08-11 |
| v0.2.0 | Cross-process hash consistency fix + complete Reader initialization | Folded into v0.3.0; not tagged separately |
| v0.3.0 | Full featload CLI (`-source`), `pkg/shm` split, `GET_STATUS` state | Tagged 2026-08-25; includes breaking API + wire-format changes |
| v1.0.0 | Phase 1 + Phase 2 hot-swap API freeze | Freeze after `Reader` / `OpWatch` shape is stable |
| v1.x | Backward compatible | — |
| v2.0.0 | Reserved for later incompatible changes | Was previously "Phase 2"; Phase 2 now targets v1.0 |

## Contributing

- Want to pick up a feature? Open an issue or PR describing your plan
- Submit designs for each phase using the [design template](TEMPLATE.md)
- Major changes must be registered as decisions in [ADRs.md](ADRs.md)
