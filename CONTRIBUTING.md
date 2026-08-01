# Contributing to featcache

Thanks for your interest in featcache! We welcome contributions of all kinds. Please read this guide before submitting.

> **Using an AI coding assistant?** Please also read [AI_CONTRIBUTING.md](AI_CONTRIBUTING.md) — AI-assisted contributions are held to the same engineering standards as human-written code.

## Table of contents

- [Ways to contribute](#ways-to-contribute)
- [Development environment](#development-environment)
- [Code standards](#code-standards)
- [Branch naming](#branch-naming)
- [Commit messages](#commit-messages)
- [Pull request workflow](#pull-request-workflow)
- [Code review](#code-review)
- [Testing requirements](#testing-requirements)
- [Documentation requirements](#documentation-requirements)

## Ways to contribute

There are many ways to get involved:

- **Report bugs**: open an issue using the [bug report template](.github/ISSUE_TEMPLATE/bug_report.md)
- **Suggest features**: use the [feature request template](.github/ISSUE_TEMPLATE/feature_request.md)
- **Propose designs**: use the [design proposal template](.github/ISSUE_TEMPLATE/design_proposal.md) and the [design template](docs/design/TEMPLATE.md)
- **Submit code**: fork → branch → PR
- **Improve documentation**: fix typos, add examples, clarify explanations

## Development environment

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.25+ | Compile and test |
| golangci-lint | v2.x | Code quality checks |
| git | any | Version control |

### Setup

```bash
# 1. Fork and clone
git clone git@github.com:<your-username>/featcache.git
cd featcache

# 2. Verify your environment
make check    # or run the steps individually
```

### Common commands

```bash
make build      # compile
make test       # tests (with race detector)
make coverage   # coverage
make lint       # golangci-lint
make vet        # go vet
make check      # all quality gates
make bench      # benchmarks
```

## Code standards

### Formatting

- Must pass `gofmt` (this project uses `gofmt -s`)
- Must pass `goimports` (local package prefix `github.com/hengli-coder/featcache`)

### Lint

- Must pass `golangci-lint run ./...` (0 issues)
- Key linters: `govet`, `staticcheck`, `errcheck`, `gosec`, `revive`, `gocritic`

### Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- All exported identifiers need godoc comments (starting with the identifier name, ending with a period)
- Error handling: never swallow errors (`_ =` only in clearly justified cases); wrap errors with `%w`
- Naming: follow Go conventions (camelCase, capitalized acronyms like `ID`/`URL`)
- Avoid unnecessary `interface{}` (use concrete types or generics)

### Platform compatibility

The project uses build tags to separate platforms:

```go
//go:build linux     — real shared memory implementation
//go:build !linux    — stubs (return ErrNotSupported)
```

- When modifying the Linux implementation, keep the stub implementation in sync
- New platform-specific code must compile and be testable on non-Linux platforms

## Branch naming

```
<type>/<short-description>

Examples:
fix/hash-seed-consistency
feat/hot-swap
docs/architecture-overview
test/loader-coverage
chore/dependabot-config
```

| Prefix | Purpose |
|--------|---------|
| `feat/` | New features |
| `fix/` | Bug fixes |
| `docs/` | Documentation |
| `test/` | Tests |
| `refactor/` | Refactoring |
| `perf/` | Performance |
| `chore/` | Miscellaneous |

## Commit messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[body]

[footer]
```

### Type

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation |
| `test` | Tests |
| `refactor` | Refactoring |
| `perf` | Performance optimization |
| `chore` | Build/tooling/dependencies |
| `ci` | CI configuration |
| `style` | Formatting (no logic change) |

### Example

```
feat(loader): share hash seed via segment header

Readers now derive the hash seed from the segment header instead of
generating a process-local seed, making hashes consistent across
processes (see ADR-6).

Closes #42
```

### Rules

- First line ≤ 72 characters
- Imperative present tense (`add`, `fix` — not `added`/`fixed`)
- Breaking changes: add `BREAKING CHANGE:` in the footer

## Pull request workflow

1. **Create a branch** from the latest `main`
2. **Develop** following the code standards and testing requirements
3. **Verify locally** — everything must pass before submitting:
   ```bash
   make check
   ```
4. **Push and open a PR** using the [PR template](.github/PULL_REQUEST_TEMPLATE.md)
5. **Wait for review** — maintainers review within 3 business days
6. **Address feedback** — update and push again

### PR requirements

- [ ] Describe what changed and why
- [ ] Reference related issues (`Closes #xx`)
- [ ] Mark the change type (feat/fix/...)
- [ ] List the tests you ran and their results
- [ ] Note the coverage impact
- [ ] Mark whether this is a breaking change
- [ ] Update related documentation

### When a PR is not needed

- Fixing comment typos (direct PR is fine, no issue needed)
- Refactoring with no behavior change (still requires full testing)

## Code review

### Review requirements

- At least 1 maintainer approval
- CI fully green (lint / test / coverage / build / security)
- Coverage must not decrease (see [Coverage strategy](#coverage-strategy))

### What reviewers look for

- Correctness: edge cases, error handling, concurrency safety
- Performance: no overhead added to the hot path (read path is lock-free and syscall-free)
- Platform compatibility: compiles and tests on non-Linux platforms
- Documentation: behavior changes are reflected in docs

## Testing requirements

### Coverage strategy

- **Total coverage ≥ 70%** (enforced by CI)
- New code must not lower total coverage
- Core modules (`loader.go`, `hashtable.go`, `reader.go`, `server.go`) target ≥ 80%

```bash
# Check locally
make coverage
# Function-level coverage
go tool cover -func=coverage.out
```

### Test conventions

- New features ship with unit tests
- Bug fixes ship with regression tests
- Prefer table-driven tests
- Test platform-independent logic with in-memory segments (`newTestSegment`)
- UDS tests use short paths (macOS 103-byte path limit)

### Benchmarks

Performance-sensitive code should provide benchmarks:

```go
func BenchmarkHashTableGet(b *testing.B) { ... }
```

## Documentation requirements

- Behavior changes must update the docs
- New exported APIs must update godoc comments
- Major design changes use the [design template](docs/design/TEMPLATE.md) and register the decision in [ADRs.md](docs/design/ADRs.md)
- New features update [README.md](README.md) and [CHANGELOG.md](CHANGELOG.md)

## Other

- Code of Conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- Security issues: **do not disclose publicly** — report privately per [SECURITY.md](SECURITY.md)
- Questions? Open an issue or a discussion
