# featcache — AI Agent Development Workflow

This file defines the workflow that AI agents must follow when developing in this repository. All AI-generated code must comply with [AI_CONTRIBUTING.md](../AI_CONTRIBUTING.md).

## Before coding

The AI **must**:

1. **Understand the architecture docs**
   - [docs/architecture/overview.md](../docs/architecture/overview.md) — roles and data flow
   - [docs/architecture/memory-layout.md](../docs/architecture/memory-layout.md) — memory layout
   - [docs/architecture/concurrency.md](../docs/architecture/concurrency.md) — concurrency model (lock-free, syscall-free read path)
   - [docs/design/ADRs.md](../docs/design/ADRs.md) — design decisions already made

2. **Review existing implementation patterns**
   - Follow the naming, comment, and error-handling style of similar code in `pkg/featcache/`
   - Note the build tag convention (`linux` / `!linux`)

3. **Identify related tests**
   - Find the test files for affected modules (`*_test.go`)
   - Confirm the test style (table-driven, `newTestSegment` in-memory segment helper)

4. **Understand API compatibility requirements**
   - Do not break existing exported APIs (breaking changes require discussion)
   - Versioning rules in [CHANGELOG.md](../CHANGELOG.md)

## During coding

The AI **must**:

1. **Follow existing project conventions**
   - gofmt -s formatting
   - godoc comments (exported identifiers)
   - `%w` error wrapping
   - No locks or syscalls on the read path

2. **Avoid unnecessary refactoring**
   - Minimal diff; only touch code within the task scope
   - Do not use unapproved skills/tools (see [skill-lock.json](skill-lock.json))

3. **Add tests alongside new features**
   - Cover normal, error, and edge paths
   - Test platform-independent logic with in-memory segments

4. **Update documentation when behavior changes**
   - godoc comments, README, docs/, CHANGELOG

## Before submission

The AI **must**, in order:

```bash
# 1. Build
go build ./...

# 2. All tests (with race detector)
go test ./... -count=1 -race

# 3. Coverage verification (≥ 70%)
go test ./... -coverprofile=coverage.out -covermode=atomic -count=1
bash scripts/check_coverage.sh coverage.out 70

# 4. Lint (0 issues)
golangci-lint run ./...

# 5. Static analysis
go vet ./...

# 6. License header check
bash scripts/check_license.sh
```

Then:

5. **Human review of the changes** — a human must review all AI-generated code
6. **Generate a modification summary** — the PR description explains what changed, test results, and AI usage scope

## Prohibited

- ❌ Unrequested large-scale refactoring
- ❌ Editing `.golangci.yml` to mask lint warnings
- ❌ Deleting/disabling tests to pass CI
- ❌ Adding unreviewed third-party dependencies
- ❌ Introducing locks or syscalls on the read path (`reader.go`, `hashtable.go`) — violates the architecture
- ❌ Changes outside the scope of the assigned task

## Verification

Any AI-assisted PR is validated by CI to the same standards (lint, test, coverage, security). The AI cannot bypass these checks.
