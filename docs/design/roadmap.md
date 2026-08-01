# Roadmap

featcache's feature evolution roadmap, organized by phase. Each phase delivers a shippable milestone.

## Phase 1: Core (current, v0.x)

**Goal**: a usable, testable, deployable core caching system.

| Feature | Status |
|---------|--------|
| Segment management (create/open/close/destroy) | ✅ |
| Header read/write | ✅ |
| HashTable (full 64-bit hash, open addressing) | ✅ |
| Loader (batch loading) | ✅ |
| Reader (zero-copy reads) | ✅ |
| UDS control-plane protocol (`OpGetInfo` / `OpGetStatus`) | ✅ |
| DataSource abstraction + built-ins (file / line / map) | ✅ |
| Tests + benchmarks | ✅ |

**To fix**:

- [ ] Cross-process hash seed consistency (see [ADR-6](ADRs.md#adr-6-why-hashmaphash-for-hashing)) — share the seed via the Header
- [ ] `Reader.connect` should actually send GET_INFO (currently opens the segment directly)
- [ ] `featload` daemon should accept a `-source` data source flag (currently only creates an empty segment)

## Phase 2: Hot swap

**Goal**: replace data at runtime without interrupting service.

| Feature | Status |
|---------|--------|
| Double-buffered version switching | planned |
| `OpWatch` protocol | planned |
| Incremental updates + diff detection | planned |
| Old-segment reference counting and reclamation | planned |

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
| v0.1.0 | First usable Phase 1 release | Initial release |
| v0.2.0 | Cross-process hash consistency fix + complete Reader initialization | Behavior change |
| v0.3.0 | Full featload CLI (`-source` etc.) | Feature completion |
| v1.0.0 | Phase 1 stable, API frozen | First stable release |
| v1.x | Backward compatible | — |
| v2.0.0 | Phase 2 hot swap | May introduce API changes |

## Contributing

- Want to pick up a feature? Open an issue or PR describing your plan
- Submit designs for each phase using the [design template](TEMPLATE.md)
- Major changes must be registered as decisions in [ADRs.md](ADRs.md)
