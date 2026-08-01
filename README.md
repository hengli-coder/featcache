# featcache

> Zero-copy runtime data cache for AI inference.
> Load once. Share across processes with zero-copy reads. Hot-swap ready.

[![Go Version](https://img.shields.io/github/go-mod/go-version/hengli-coder/featcache)](https://github.com/hengli-coder/featcache)
[![Go Report Card](https://goreportcard.com/badge/github.com/hengli-coder/featcache)](https://goreportcard.com/report/github.com/hengli-coder/featcache)
[![CI](https://github.com/hengli-coder/featcache/actions/workflows/ci.yml/badge.svg)](https://github.com/hengli-coder/featcache/actions/workflows/ci.yml)
[![Release](https://github.com/hengli-coder/featcache/actions/workflows/release.yml/badge.svg)](https://github.com/hengli-coder/featcache/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

[中文文档](README.zh-CN.md)

---

## What is featcache?

**featcache** is a zero-copy runtime data cache for AI inference. It solves one problem:

> Inference processes re-load large static datasets (embeddings, tokenizer vocabularies, feature dictionaries) on every startup — slow startup, wasted memory.

With a traditional setup, N inference processes each load their own copy of 10GB+ of data. featcache loads the data **once**, writes it into a POSIX shared memory segment, and lets every process read it directly — zero-copy, lock-free, syscall-free.

```
┌───────────────────────┐   load once   ┌───────────────────────┐
│  Loader (featload)    │ ────────────► │  Shared Memory Segment │
│  • reads DataSource   │               │  [Header]              │
│  • writes segment     │               │  [Hash Index]          │
│  • builds hash index  │               │  [Data Region]         │
│  • serves UDS control │               └──────────┬────────────┘
└───────────────────────┘                          │ mmap
                                                   ▼
                            ┌────────────────────────────────────┐
                            │ Inference proc 1   proc 2   ... N  │
                            │ Read directly from shared memory   │
                            │ Zero-copy · no locks · no syscalls │
                            └────────────────────────────────────┘
```

### Key features

- **Zero-copy reads** — clients read directly from shared memory; lookups are plain memory accesses
- **Load once, share everywhere** — N processes share a single copy of the data
- **Instant startup** — inference processes just `mmap`; startup cost is independent of data size
- **Compact storage** — append-only data region with no internal fragmentation
- **Pure Go** — the only external dependency is `golang.org/x/sys` (for `mmap`)
- **Hot swap** *(Phase 2)* — replace data at runtime without restarting services

### Use cases

| Scenario | Data examples | Typical size |
|----------|---------------|--------------|
| Recommendation systems | User/item embedding tables | 10GB – 100GB |
| LLM inference | Tokenizer vocabularies, BPE encodings | 1GB – 10GB |
| Multimodal models | Image/text feature dictionaries | 5GB – 50GB |
| Ad CTR prediction | Sparse feature dictionaries, lookup tables | 10GB – 30GB |
| RAG systems | Document embedding stores | 10GB – 100GB |
| Search engines | ANN indexes, inverted indexes | 5GB – 50GB |

---

## Installation

### Prerequisites

- Go 1.25+
- Linux (POSIX shared memory + `mmap`; see [Platform support](#platform-support))

### Install the loader daemon

```bash
go install github.com/hengli-coder/featcache/cmd/featload@latest
```

Or download a pre-built binary from the [Releases](https://github.com/hengli-coder/featcache/releases) page.

### Use as a library

```bash
go get github.com/hengli-coder/featcache
```

---

## Quick start

### 1. Load data and start the daemon

The `featload` daemon creates a shared memory segment and serves the UDS control plane. Data loading is done through the `Loader` API (below) — a `-source` CLI flag is planned (see [roadmap](docs/design/roadmap.md)).

```bash
# Build
go build ./cmd/featload

# Start the daemon (segment "my-embeddings", 2 GB by default)
featload -name my-embeddings -size 10737418240

# Options
featload -name featcache -size 2147483648 -uds '\x00featcache' -version
```

### 2. Load data with the Loader API

```go
import "github.com/hengli-coder/featcache/pkg/featcache"

loader, err := featcache.NewLoader(featcache.LoaderConfig{
    SegmentName: "my-embeddings",
    SegmentSize: 10 << 30, // 10 GB
})
if err != nil { /* handle */ }
defer loader.Destroy()

if err := loader.Init(10_000_000); err != nil { /* handle */ } // pre-size hash table

// From a binary file
ds := featcache.NewFileDataSource("/data/embeddings.bin")
count, err := loader.Load(ds) // Load calls ds.Open() internally
if err != nil { /* handle */ }

// Or from an in-memory map (tests / demos)
ds2 := featcache.NewMapDataSource(map[string][]byte{"key": []byte("value")})
count, err = loader.Load(ds2)
```

### 3. Read from inference processes

```go
// Initialize the Reader — one UDS round-trip for metadata, then pure shared memory.
reader, err := featcache.NewReader("my-embeddings", "\x00featcache")
if err != nil { /* handle */ }
defer reader.Close()

// Lookup (memory-speed)
embedding, ok := reader.Get([]byte("user_embedding_123"))
if !ok { /* miss */ }

// Batch lookup
keys := [][]byte{[]byte("user_1"), []byte("user_2")}
values, results := reader.GetBatch(keys)
```

> **Warning**: the byte slice returned by `Get` is a view into shared memory — **do not modify it**. Copy it first if you need to mutate it.

### 4. Run the demo (Linux)

```bash
go run ./examples/featload-demo
```

The demo loads a small dataset via `MapDataSource`, then reads it back zero-copy in the same process.

---

## Configuration

### featload CLI

| Flag | Default | Description |
|------|---------|-------------|
| `-name` | `featcache` | Shared memory segment name |
| `-size` | `2GB` | Segment size in bytes |
| `-uds` | `\x00featcache` | UDS address (`\x00` prefix = abstract namespace) |
| `-version` | `false` | Print version info and exit |

### LoaderConfig

| Field | Default | Description |
|-------|---------|-------------|
| `SegmentName` | — | Shared memory segment name |
| `SegmentSize` | `2GB` | Segment size in bytes |
| `LoadFactor` | `0.5` | Hash table load factor (0.0–1.0) |

### Built-in data sources

| Source | Format |
|--------|--------|
| `NewFileDataSource(path)` | Binary: `[keyLen:4B LE][key][valLen:4B LE][val]` per entry |
| `NewLineDataSource(path)` | Text: one `key\tvalue` line per entry |
| `NewMapDataSource(map)` | In-memory map (tests / demos) |

Implement the [DataSource](pkg/featcache/datasource.go) interface for custom sources (database, object store, stream).

---

## Architecture

featcache separates the **control plane** from the **data plane**:

- **Control plane** — Unix Domain Socket, used only for initialization and metadata (`GET_INFO` / `GET_STATUS`)
- **Data plane** — shared memory; every data read happens here, never over UDS

```
Loader starts → reads DataSource → writes segment → ready
                                              ↓
Inference process starts → UDS metadata → mmap segment → query directly
                                              ↓
            All GET operations go through shared memory, never UDS
```

Documentation:

- [Architecture overview](docs/architecture/overview.md)
- [Memory layout](docs/architecture/memory-layout.md)
- [Concurrency model](docs/architecture/concurrency.md)
- [Control-plane protocol](docs/architecture/control-plane.md)
- [Design documents](docs/design/)
- [Architecture Decision Records (ADRs)](docs/design/ADRs.md)

### Platform support

Linux only. The codebase uses build tags:

- `//go:build linux` — real implementation (`/dev/shm` + `mmap` via `golang.org/x/sys/unix`)
- `//go:build !linux` — stubs returning `ErrNotSupported`

Core logic is tested with in-memory byte slices, so tests run on any platform (including macOS).

---

## Project layout

```
featcache/
├── cmd/
│   └── featload/           # Loader daemon entry point
├── pkg/
│   └── featcache/          # Core library
│       ├── types.go        # Header, HashSlot, constants, OpCodes
│       ├── hash.go         # HashKey (hash/maphash)
│       ├── segment.go      # Segment API (platform-independent)
│       ├── segment_linux.go# Linux mmap implementation
│       ├── segment_other.go# Non-Linux stubs
│       ├── hashtable.go    # Open-addressed hash table
│       ├── loader.go       # Loader (write side)
│       ├── reader.go       # Reader (zero-copy read side)
│       ├── server.go       # UDS control-plane server
│       ├── datasource.go   # DataSource interface + built-ins
│       ├── protocol.go     # UDS binary protocol codec
│       └── *_test.go       # Tests
├── examples/
│   └── featload-demo/      # End-to-end demo
└── docs/                   # Architecture + design docs
```

---

## Development

### Requirements

- Go 1.25+
- Linux (core functionality) / macOS (testing, development)

### Common commands

```bash
# Build
make build

# Test (with race detector)
make test

# Coverage
make coverage

# Lint
make lint

# Everything (fmt, vet, lint, test, coverage, license)
make check
```

Or use Go directly:

```bash
# Run tests
go test ./pkg/featcache/ -v -count=1

# Race detector
go test ./pkg/featcache/ -v -race -count=1

# Benchmarks
go test ./pkg/featcache/ -bench=. -benchmem -count=1

# Coverage
go test ./pkg/featcache/ -coverprofile=coverage.out -covermode=atomic -count=1
go tool cover -func=coverage.out
```

### Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the contribution workflow, code standards, and commit requirements.

### Using AI coding assistants?

See [AI_CONTRIBUTING.md](AI_CONTRIBUTING.md) for AI-assisted contribution rules and disclosure requirements.

---

## Performance

| Metric | Target | Notes |
|--------|--------|-------|
| Inference process startup | < 100 ms | Independent of data size (mmap + one metadata lookup) |
| Single lookup | < 100 ns | 1–2 atomic reads + hash comparison, no syscalls |
| Batch lookup | N × single | Linear scaling, no extra overhead |
| Memory efficiency | ~1.002 × data size | Compact storage, minimal index overhead |
| Multi-process memory savings | (N−1) × data size | N processes share one copy |

---

## Comparison

| Solution | Zero-copy multi-proc sharing | Lookup latency | Hot update | 10GB+ optimized | External deps |
|----------|------------------------------|----------------|------------|-----------------|---------------|
| **featcache** | ✅ | < 100 ns | ✅ (Phase 2) | ✅ | None |
| Redis | ❌ network | ~100 µs | ✅ | ❌ | None |
| FAISS | ⚠️ mmap sharing | < 100 ns | ❌ | ✅ | C++ |
| Plasma (deprecated) | ✅ | < 100 ns | ❌ | ❌ | C++ |
| Safetensors | ❌ per-process mmap | < 100 ns | ❌ | ✅ | Python/C++ |

---

## Roadmap

- [ ] **Phase 1 (current)**: core — Segment, HashTable, Loader, Reader, UDS control plane, DataSource abstraction
- [ ] **Phase 2**: hot swap — double-buffered version switching, `WATCH_VERSION`, incremental updates
- [ ] **Phase 3**: enhancements — multi-tier storage, persistence, metrics

See [docs/design/roadmap.md](docs/design/roadmap.md).

---

## Security

Found a security issue? Read [SECURITY.md](SECURITY.md) and **do not** disclose it publicly (GitHub Issues, discussions, etc.).

---

## Code of Conduct

By participating in this project, you agree to the [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

---

## License

[Apache License 2.0](LICENSE)
