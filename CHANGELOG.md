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

### Added

- New `MapDataSource` in-memory data source (`NewMapDataSource`)
- Tests for `Loader`, `CacheServer` UDS protocol, and data sources
- Example program [examples/featload-demo](examples/featload-demo)
- Architecture and design documentation (`docs/architecture/`, `docs/design/`)
- Makefile, coverage threshold script, CI coverage gate
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

### Fixed

- Fixed nested check in `LineDataSource.Next` (nestingReduce)
- Fixed byte comparison in `featcache_test.go` (stringXbytes)
- Fixed all golangci-lint findings (gocritic, godot, gofmt, revive, unconvert)

### Security

- Added `SECURITY.md` vulnerability reporting and disclosure process

## [0.1.0] - 2026-07-19 (initial implementation)

### Added

- Core shared memory segment management (create/open/close/destroy)
- Open-addressed hash table (full 64-bit hash, CAS writes, atomic reads)
- Loader batch loading (DataSource abstraction)
- Reader zero-copy reads (Get / GetBatch / GenCounter)
- UDS control-plane protocol (GET_INFO / GET_STATUS)
- Data source implementations: FileDataSource, LineDataSource
- Performance benchmarks
- Apache License 2.0

---

*CHANGELOG entries are assembled by maintainers from PRs at each release. Contributors may suggest CHANGELOG entries in their PR descriptions.*
