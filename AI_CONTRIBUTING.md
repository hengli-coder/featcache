# AI Contribution Guide

This project allows contributors to use AI coding assistants (Claude Code, Copilot, Cursor, etc.). **AI-generated code must meet the same engineering standards as human-written code.**

> 中文版见 [AI_CONTRIBUTING.zh-CN.md](AI_CONTRIBUTING.zh-CN.md)。

## 1. Scope

This guide applies to all AI-assisted contributions:

- Code generation (features, bug fixes)
- Refactoring
- Test generation
- Documentation generation
- Config/script generation

**Regardless of who writes the code, the contributor bears full responsibility.**

## 2. AI Usage Disclosure

Contributors should disclose significant AI assistance in the PR description (see [PR template](.github/PULL_REQUEST_TEMPLATE.md)):

```markdown
## AI-assisted contribution

- [x] AI assistant used (tool: Claude Code)
- Scope:
  - Feature code: loader.go xxx function
  - Tests: loader_test.go
  - Docs: docs/architecture/overview.md
- Human review: all AI-generated code reviewed
```

| Scenario | Disclose? |
|----------|-----------|
| Code generation | ✅ Required |
| Refactoring | ✅ Recommended |
| Test generation | ✅ Recommended |
| Documentation | ✅ Recommended |
| Minor edits (typos) | ❌ Not needed |

## 3. AI-Generated Code Requirements

AI-generated code must:

1. **Follow project coding standards** — see [CONTRIBUTING.md](CONTRIBUTING.md) (gofmt, golangci-lint, naming, comments)
2. **Pass all existing tests** — `go test ./...`
3. **Include appropriate tests** — new features ship with unit tests
4. **Not reduce test coverage** — total coverage ≥ 70%
5. **Pass all linters** — `golangci-lint run ./...` (0 issues)
6. **Pass security checks** — `govulncheck ./...`, gosec
7. **Follow architecture guidelines** — see [docs/architecture/](docs/architecture/) and [docs/design/](docs/design/)
8. **Avoid unnecessary refactoring** — minimal diff
9. **Maintain API compatibility** — breaking changes require discussion and a `BREAKING CHANGE:` marker

### Prohibited

- ❌ Bypassing tests (deleting tests, masking issues with `//nolint`)
- ❌ New logic without tests
- ❌ Designs that conflict with the project architecture (e.g. locks on hot paths)
- ❌ Unreviewed third-party dependencies
- ❌ Editing `.golangci.yml` to mask warnings (legitimate relaxations need justification)

## 4. Mandatory Validation Before Submission

Before submitting any AI-assisted contribution, **all** of the following must succeed:

```bash
# 1. All tests (with race detector)
go test ./... -count=1 -race

# 2. Coverage
go test ./... -coverprofile=coverage.out -covermode=atomic -count=1
bash scripts/check_coverage.sh coverage.out 70

# 3. Lint
golangci-lint run ./...

# 4. Static analysis
go vet ./...
```

**Do NOT submit a PR if:**

| Condition | Action |
|-----------|--------|
| Tests fail | Fix first |
| Coverage decreased | Add tests |
| Linter has unresolved issues | Fix |
| Build fails | Fix |
| Security checks fail | Fix |

## 5. AI Agent Workflow

### Before coding

The AI must:

1. **Understand the architecture docs** — [docs/architecture/overview.md](docs/architecture/overview.md) and [docs/design/](docs/design/)
2. **Review existing implementation patterns** — follow the conventions of similar code
3. **Identify related tests** — find the tests for affected modules
4. **Understand API compatibility requirements** — see versioning rules in [CHANGELOG.md](CHANGELOG.md)

### During coding

The AI must:

1. **Follow existing project conventions** — consistent naming, comments, error handling
2. **Avoid unnecessary refactoring** — minimal diff
3. **Add tests with new functionality** — cover normal, error, and edge paths
4. **Update documentation when behavior changes** — README, docs, godoc

### Before submission

The AI must:

1. Run all tests (Section 4)
2. Run the linter
3. Verify coverage
4. **Have a human review the changes** (human-in-the-loop)
5. Generate a summary of the modifications (PR description)

## 6. AI Tool & Skill Governance (.ai/)

The project declares approved AI skills and tools via [.ai/skill-lock.json](.ai/skill-lock.json). AI assistants should:

- Only use skills listed in skill-lock.json
- Follow the declared validation steps
- Not use unapproved tools or automation

## 7. Responsibility & Review

- **Contributor responsibility**: AI-generated code is treated as the contributor's own code — full responsibility
- **Maintainer review**: maintainers review at normal standards; AI generation does not lower the bar
- **False disclosure**: hiding AI usage that causes problems is handled per normal contribution rules

---

*Questions or suggestions? Open a [discussion](https://github.com/hengli-coder/featcache/discussions).*
