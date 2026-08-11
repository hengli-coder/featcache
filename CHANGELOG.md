# Changelog

This project follows [Semantic Versioning](https://semver.org/) and the [Keep a Changelog](https://keepachangelog.com/) format.

## Versioning rules

- **MAJOR**: breaking changes (API incompatibility)
- **MINOR**: backward-compatible new features
- **PATCH**: backward-compatible bug fixes

### Breaking change policy

Breaking changes include:

- Changes to exported API signatures
- Behavior changes that break existing usage
- Removal of exported symbols
- Memory layout changes (affect persistence/shared data)

Breaking changes must be marked `BREAKING CHANGE:` in the PR and the migration path must be described in the corresponding CHANGELOG entry.

## Change categories

| Category | Description |
|----------|-------------|
| `Added` | New features |
| `Changed` | Changes to existing features |
| `Deprecated` | Features about to be removed |
| `Removed` | Removed features |
| `Fixed` | Bug fixes |
| `Security` | Security fixes (prefixed with `[SECURITY]`) |

## [Unreleased]

## [0.1.0] - 2026-08-11

### Added

- Core shared memory segment management (create/open/close/destroy)
- Open-addressed hash table (full 64-bit hash, CAS writes, atomic reads)
- Loader batch loading (DataSource abstraction)
- Reader zero-copy reads (Get / GetBatch / GenCounter)
- UDS control-plane protocol (GET_INFO / GET_STATUS)
- Data source implementations: FileDataSource, LineDataSource, `NewMapDataSource`
- `featload` daemon, including a `-source` data source flag
- Performance benchmarks
- Apache License 2.0
- Tests for `Loader`, `CacheServer` UDS protocol, and data sources
- Example program [examples/featload-demo](examples/featload-demo)
- Architecture and design documentation (`docs/architecture/`, `docs/design/`), including ADRs and a roadmap
- Makefile, coverage threshold script, CI coverage gate, Docker-based Linux test target
- Dependabot configuration and dependency vulnerability scanning
- GitHub issue/PR templates
- AI contribution governance (`AI_CONTRIBUTING.md`, `.ai/skill-lock.json`)
- Dockerfile and dev container configuration

### Changed

- `featload` removed the unused `fmt` reference; added `-version` flag and version injection via ldflags
- Normalized return value names on `Reader.connect` and `GetBatch`
- Normalized return value names on `FileDataSource.Next` / `LineDataSource.Next`
- `CacheServer.Listen` uses the standard octal literal `0o777`; normalized empty-string checks
- `segment_other.go` non-Linux stub supports in-memory segment close/destroy (test-friendly)
- Documentation restructured: README is user-oriented; `docs/` covers architecture and design
- Repository renamed from `shm-go` to `featcache` to match the Go module path and product name
- **BREAKING CHANGE**: extracted the generic shared-memory segment primitive
  out of `pkg/featcache` into its own package, `pkg/shm` (same module, same
  repo — see [ADR-7](docs/design/ADRs.md#adr-7-why-pkgshm-is-a-separate-package-not-a-separate-repo)).
  `featcache.Segment`, `featcache.CreateSegment`, `featcache.OpenSegment`,
  and `featcache.ErrNotSupported` moved to `shm.Segment`, `shm.CreateSegment`,
  `shm.OpenSegment`, and `shm.ErrNotSupported`. `Loader.Segment()`,
  `Reader.Segment()`, `CacheServer.Segment()`, `NewServer`, and
  `NewReaderFromSegment` now take/return `*shm.Segment`. Migration: replace
  `featcache.Segment`-typed variables with `*shm.Segment` and import
  `github.com/hengli-coder/featcache/pkg/shm`; the removed
  `Segment.Header()`/`HashOffset()`/`HashCap()`/`DataOffset()`/`GenCounter()`
  accessors had no callers outside the package and have no replacement.

### Fixed

- Fixed nested check in `LineDataSource.Next` (nestingReduce)
- Fixed byte comparison in `featcache_test.go` (stringXbytes)
- Fixed all golangci-lint findings (gocritic, godot, gofmt, revive, unconvert)
- **BREAKING CHANGE**: `HashKey`/`HashKeyWithSeed` no longer use `hash/maphash`.
  `maphash.Seed` cannot be shared across processes (the runtime mixes in a
  process-local random key that the `Seed` value doesn't control), so the
  Loader and a Reader in a different OS process could compute different
  hashes for the same key and silently fail to find data the Loader wrote.
  Replaced with a seeded FNV-1a implementation with no process-local state.
  `HashKeyWithSeed`'s `seed` parameter changed type from `maphash.Seed` to
  `uint64`. See [ADR-6](docs/design/ADRs.md#adr-6-why-hashmaphash-for-hashing).
- Fixed CI coverage gate scoped to `./...` (dragged down by 0%-covered
  `cmd`/`examples` packages) instead of `./pkg/featcache/`
- Fixed `gosec` weak-RNG finding by using `crypto/rand` for the hash seed

### Security

- Added `SECURITY.md` vulnerability reporting and disclosure process

---

*CHANGELOG entries are assembled by maintainers from PRs at each release. Contributors may suggest CHANGELOG entries in their PR descriptions.*
