---
name: Pull Request
about: Submit a code change
title: ""
labels: []
assignees: ""
---

<!-- Please fill in as much of the following as possible. Items marked * are required. Run `make check` locally before submitting. -->

## Description *

Briefly describe what this PR changes and why.

## Related Issue *

- Closes #<!-- fill in the issue number -->

## Change type *

Check all that apply:

- [ ] feat: new feature
- [ ] fix: bug fix
- [ ] docs: documentation
- [ ] test: tests
- [ ] refactor: refactoring
- [ ] perf: performance optimization
- [ ] chore: build/tooling/dependencies
- [ ] ci: CI configuration
- [ ] BREAKING CHANGE: breaking change

## Testing performed *

List the tests you ran locally and their results:

```bash
go test ./... -count=1 -race          # result: ✅ passed
golangci-lint run ./...                # result: ✅ 0 issues
go vet ./...                           # result: ✅ passed
go test ./... -coverprofile=coverage.out -covermode=atomic
bash scripts/check_coverage.sh coverage.out 70   # result: ✅
```

## Coverage impact *

- Coverage before this change:
- Coverage after this change:
- New tests added: yes / no

## Breaking changes

- [ ] No
- [ ] Yes → describe the migration path:

## Documentation updates

- [ ] Not needed
- [ ] Updated README.md
- [ ] Updated docs/ (architecture/design)
- [ ] Updated CHANGELOG.md
- [ ] Updated godoc comments

## Checklist *

- [ ] Code formatted with `gofmt -s`
- [ ] Passes `golangci-lint run ./...` (0 issues)
- [ ] Passes `go vet ./...`
- [ ] All tests pass (with race detector)
- [ ] Coverage is not below the threshold (70%) and did not decrease
- [ ] New/modified code has corresponding tests
- [ ] Behavior changes are reflected in the docs
- [ ] Commit messages follow [Conventional Commits](../../CONTRIBUTING.md#commit-messages)

## AI-assisted contribution

- [ ] No
- [ ] Yes → disclose the tools and scope in the PR description (see [AI_CONTRIBUTING.md](../../AI_CONTRIBUTING.md))
