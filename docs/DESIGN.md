# DESIGN.md (superseded)

> ⚠️ **This document is out of date and superseded.**

`docs/DESIGN.md` was the original, monolithic design document for the initial prototype. It contains a **v2 memory layout sketch** (`SourceID` field, `reserved[4]`, struct layouts) that **does not match** the current implementation:

- The current Header has `SegmentID` + `Flags` + `Reserved[16]` (see [types.go](../pkg/featcache/types.go)) — there is no `SourceID`
- The `Segment`/`Loader`/`Reader` struct fields described there no longer match the code
- The ecosystem survey and Phase 2/3 planning have moved to the structured docs below

## Current documentation

Use these instead:

- [Architecture overview](../docs/architecture/overview.md) — roles, data flow, components
- [Memory layout](../docs/architecture/memory-layout.md) — exact Header/slot/data-region layout
- [Concurrency model](../docs/architecture/concurrency.md) — lock-free read path
- [Control-plane protocol](../docs/architecture/control-plane.md) — UDS protocol spec
- [Design documents](../docs/design/) — per-component designs, ADRs, roadmap

## Retention decision

The file is kept as a historical record of the original design intent, but it is **not authoritative**. If you are reading it for implementation guidance, stop and read the documents above. It may be removed once the repository history has settled.
