# Architecture Documentation

This directory contains featcache's architecture documentation: overall design, component responsibilities, data flow, and key design decisions.

> English is the authoritative version. Chinese translations live in sibling `.zh-CN.md` files.

## Contents

| Document | Description |
|----------|-------------|
| [overview.md](overview.md) | System overview, roles, component responsibilities, data flow |
| [memory-layout.md](memory-layout.md) | Shared memory segment layout: Header, hash table, data region |
| [concurrency.md](concurrency.md) | Concurrency model: single-writer/multi-reader, lock-free reads, atomicity |
| [control-plane.md](control-plane.md) | UDS control-plane protocol and communication model |

## Related documents

- [Design documents](../design/) — component design proposals and ADRs
- [CLAUDE.md](../../CLAUDE.md) — developer guidance

## Suggested reading order

1. [overview.md](overview.md) — build a mental model of the system
2. [memory-layout.md](memory-layout.md) — understand how data is stored
3. [concurrency.md](concurrency.md) — understand the concurrency safety model
4. [control-plane.md](control-plane.md) — understand the control-plane protocol

---

*Last updated 2026-08. When things change, please update [CHANGELOG.md](../../CHANGELOG.md).*
