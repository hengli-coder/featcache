# featcache — Branch Protection & Release Governance

This file documents `main` branch protection, the release process, and versioning policy. Settings are configured by repository admins on GitHub.

## 1. Branch model

```
main        ← protected: no direct pushes; merges require PR + review + CI
  │
  ├── feat/*       feature branches (merge back into main)
  ├── fix/*        bug fix branches
  ├── docs/*       documentation branches
  └── release/*    release branches (if needed)

tags:
  v0.x.y          SemVer version tags (trigger the Release workflow)
```

## 2. main branch protection rules

Configure in **Settings → Branches → Add branch protection rule**:

| Rule | Suggested value | Description |
|------|-----------------|-------------|
| Require a pull request before merging | ✅ | all changes go through PRs |
| Require approvals | 1 | at least 1 maintainer approval |
| Dismiss stale reviews | ✅ | new commits invalidate old reviews |
| Require status checks | ✅ | see "Required CI checks" below |
| Require branches to be up to date | ✅ | must be synced with main before merging |
| Do not allow bypassing the above settings | ✅ | admins cannot bypass either |
| Require signed commits | recommended | if the team enables GPG/SSH signing |
| Require linear history | recommended | keep history clean |

### Required CI checks

| Check name (job) | Description |
|------------------|-------------|
| `Lint` | golangci-lint + go vet (P0/P1 block merges) |
| `Test` | unit tests + race |
| `Coverage gate` | coverage ≥ 70%, fails below |
| `Build` (matrix) | Linux amd64 + arm64 compilation |
| `Security scan` | govulncheck + CodeQL |

> **Note**: `Benchmark` and `Quality Metrics` workflows are informational and not required checks.

## 3. Versioning policy (SemVer)

Follow [Semantic Versioning](https://semver.org/):

```
vMAJOR.MINOR.PATCH
```

| Version | Meaning | Example |
|---------|---------|---------|
| MAJOR | breaking changes | `v2.0.0` |
| MINOR | backward-compatible features | `v0.3.0` → `v1.1.0` |
| PATCH | bug fixes | `v1.1.1` |

### 0.x stage

- `v0.x.y`: within 0.x, `MINOR` bumps may include breaking changes (SemVer 0.x rules)
- Breaking changes still require a `BREAKING CHANGE:` marker in the CHANGELOG

## 4. Release process

```
1. Maintainer merges all target PRs into main
2. Verify the CHANGELOG.md [Unreleased] section is complete
3. Tag the release (triggers the Release workflow):
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
4. .github/workflows/release.yml runs automatically:
   - GoReleaser builds linux/amd64 + linux/arm64
   - generates checksums.txt
   - generates release notes (from CHANGELOG / commits)
   - publishes the GitHub Release
```

### Release checklist

- [ ] CHANGELOG.md [Unreleased] → new version number
- [ ] All CI checks pass
- [ ] No unresolved P0/P1 issues
- [ ] Version tag pushed
- [ ] Verify binaries and checksums on the Release page

## 5. Security

- Security fixes are prefixed `[SECURITY]` in the CHANGELOG
- Emergency security fixes may skip the regular release cadence (quick patch release)
- See [SECURITY.md](../SECURITY.md)

## 6. Related configuration

| File | Purpose |
|------|---------|
| [.github/workflows/ci.yml](workflows/ci.yml) | PR validation (required checks) |
| [.github/workflows/release.yml](workflows/release.yml) | release process |
| [.github/workflows/quality-metrics.yml](workflows/quality-metrics.yml) | weekly quality report |
| [.github/dependabot.yml](dependabot.yml) | dependency updates |
| [.goreleaser.yml](../../.goreleaser.yml) | release build configuration |
