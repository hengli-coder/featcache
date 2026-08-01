# Design Documents

This directory holds featcache's design documents. Each design document follows the unified template: problem, goals, non-goals, proposed design, alternatives, risks, future improvements.

> English is the authoritative language for design documents. Chinese localization may be added as `.zh-CN.md` siblings if needed.

## Contents

| Document | Description |
|----------|-------------|
| [TEMPLATE.md](TEMPLATE.md) | Design proposal template (copy this for new designs) |
| [ADRs.md](ADRs.md) | Architecture Decision Records (ADR index) |
| [roadmap.md](roadmap.md) | Roadmap and future planning |
| [segment-design.md](segment-design.md) | Shared memory segment design |
| [hashtable-design.md](hashtable-design.md) | Hash table design |
| [loader-reader-design.md](loader-reader-design.md) | Loader/Reader design |
| [control-plane-design.md](control-plane-design.md) | Control-plane protocol design |
| [hot-swap-design.md](hot-swap-design.md) | Hot-swap design (Phase 2) |

## How to contribute a design

1. Copy [TEMPLATE.md](TEMPLATE.md)
2. Fill in problem, goals, non-goals, proposed design, alternatives, risks
3. Register related decisions in [ADRs.md](ADRs.md)
4. Describe the design change in the PR description

## Suggested reading order

1. [ADRs.md](ADRs.md) — summary of core design decisions
2. [segment-design.md](segment-design.md) — the memory segment
3. [hashtable-design.md](hashtable-design.md) — the index structure
4. [loader-reader-design.md](loader-reader-design.md) — read/write paths
5. [control-plane-design.md](control-plane-design.md) — the control plane
6. [hot-swap-design.md](hot-swap-design.md) — Phase 2 hot swap
