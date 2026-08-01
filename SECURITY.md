# Security Policy

## Supported Versions

The following versions receive security updates:

| Version | Supported |
|---------|-----------|
| v0.x (current development) | ✅ Actively supported |
| Earlier versions | ❌ Not supported |

> This project has not yet released a stable v1.0.0. All 0.x versions are treated as "current development".

## Reporting a Vulnerability

**Do not** disclose security vulnerabilities publicly (GitHub Issues, discussions, or any public channel).

Please report privately through one of the following channels:

1. **Email**: send to `security@example.com` (replace with the project's actual security address)
   - Subject format: `[featcache-security] <brief description>`
   - PGP encryption is recommended (see the maintainer's profile for the key fingerprint)

2. **GitHub private vulnerability reporting** (recommended):
   - Create a private report on the repository's **Security → Report a vulnerability** page
   - GitHub private vulnerability reports are not publicly visible

### What to include

- Vulnerability description and impact
- Reproduction steps (minimal example preferred)
- Affected versions
- Possible fix suggestions (optional)

## Disclosure Process

```
T-0   : report received, validity confirmed (reply within 24 hours)
T+3d  : initial assessment (severity, impact scope)
T+7d  : fix released (if applicable) or mitigation provided
T+14d : public disclosure (after the patch is released), CVE assigned if requested
```

The timeline may be adjusted based on severity and fix complexity.

### Severity levels

| Level | Definition | Response time |
|-------|------------|---------------|
| Critical | Remote code execution, data breach | 24h |
| High | Unauthorized access, data tampering | 3d |
| Medium | Information disclosure, DoS | 7d |
| Low | Other | 14d |

## Security Best Practices

### Deployment

- **Shared memory permissions**: `/dev/shm/<segment>` defaults to 0600 — ensure only same-user processes can access it
- **UDS permissions**: filesystem UDS defaults to `chmod 0777`; tighten in production (e.g. 0755 or per-user isolation)
- **Same-user deployment**: the Loader and inference processes should run as the same system user
- **Multi-tenant environments**: do not share one featcache instance across tenants

### Development

- New code must pass `gosec` security checks (enforced by CI)
- Dependency vulnerabilities are monitored by Dependabot + `govulncheck`
- Shared memory bounds checks: all out-of-bounds reads must return an error, never panic
- Never log sensitive data (embedding contents, etc.)

### Known risks and mitigations

| Risk | Mitigation |
|------|------------|
| Shared memory segment name collision | `O_EXCL` creation + unique segment names |
| Segment residue after crash | Explicit `Destroy()`; ops scripts as fallback |
| Cross-process hash seed inconsistency | See [ADR-6](docs/design/ADRs.md), being fixed |
| UDS abstract namespace exposure | Same-user namespace only; restrict in sensitive environments |

## Dependency Security

- Dependabot checks for dependency updates weekly
- Run `govulncheck ./...` before each release to verify no known vulnerabilities
- Only reviewed dependencies are allowed (currently only `golang.org/x/sys`)

## Security Updates

Security fixes are released under the following rules:

- **Critical/High**: released as a standalone patch as soon as possible
- **Medium/Low**: released with the next regular version
- All security fixes are marked with the `[SECURITY]` prefix in [CHANGELOG.md](CHANGELOG.md)
