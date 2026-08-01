# Control-Plane Protocol (UDS)

The control plane communicates over a Unix Domain Socket. It is used **only** for initialization, metadata queries, and version notification. **All data reads go through shared memory, never over UDS.**

## 1. Design principles

- **Control plane / data plane separation**: UDS is only for initialization and metadata
- **Zero-copy data plane**: all data reads go through shared memory
- **Lightweight protocol**: connect-and-use, no handshake overhead
- **Abstract namespace**: defaults to a `\x00`-prefixed abstract socket (no filesystem entry)

## 2. OpCode definitions

### Implemented in Phase 1

| OpCode | Value | Description |
|--------|-------|-------------|
| `OpGetInfo` | `0x01` | Get segment metadata (name, size, layout) |
| `OpGetStatus` | `0x02` | Get loader status |

### Reserved for Phase 2/3

| OpCode | Value | Description |
|--------|-------|-------------|
| `OpWatch` | `0x03` | Watch for version-change notifications |
| `OpPin` | `0x04` | Pin data in memory (multi-tier storage) |
| `OpPrefetch` | `0x05` | Prefetch data into a cache tier |
| `OpEvict` | `0x06` | Evict cached data |
| `OpList` | `0x07` | List loaded datasets |
| `OpReload` | `0x08` | Trigger a reload |

## 3. Request format

```
OpCode:   1B
KeyLen:   2B (uint16, big-endian)
Body:     key bytes (KeyLen)
```

| Field | Size | Description |
|-------|------|-------------|
| OpCode | 1B | Operation code |
| KeyLen | 2B | Key length (big-endian) |
| Key | KeyLen | Request body (variable) |

## 4. Response format

```
Status:      1B
SegmentName: 64B (fixed-length, null-padded)
SegmentSize: 8B  (uint64, big-endian)
HashOffset:  4B  (uint32, big-endian)
HashCap:     4B  (uint32, big-endian)
DataOffset:  4B  (uint32, big-endian)
GenCounter:  8B  (uint64, big-endian)
```

### Status values

| Value | Constant | Description |
|-------|----------|-------------|
| `0x00` | `RespOK` | Success |
| `0x01` | `RespNotFound` | Not found |
| `0x02` | `RespBusy` | Loading / reloading |
| `0x03` | `RespError` | Generic error |

## 5. Client initialization flow

```
Inference process
  │
  ├─ 1. Connect to UDS (abstract namespace, e.g. "\x00featcache-<name>")
  ├─ 2. Send GET_INFO request
  ├─ 3. Receive the response with segment metadata (name, size, HashOffset, HashCap, DataOffset, GenCounter)
  ├─ 4. mmap the shared memory segment
  ├─ 5. Close the UDS connection
  └─ 6. All subsequent queries go through shared memory; UDS is no longer used
```

> **Note on the current implementation**: `Reader.connect` currently opens the segment directly; sending a real GET_INFO request is a planned TODO (see [roadmap](../design/roadmap.md)).

## 6. Implementation locations

| Content | Location |
|---------|----------|
| Protocol codec | `pkg/featcache/protocol.go` |
| OpCode / StatusCode definitions | `pkg/featcache/types.go` |
| Server | `pkg/featcache/server.go` |
| Client connection | `pkg/featcache/reader.go` |

## 7. Security notes

- The UDS abstract namespace is only reachable within the same user namespace
- Filesystem UDS (`/`-prefixed paths) defaults to `chmod 0777`; adjust permissions per deployment in production
- Control access to `/dev/shm/<segment>` via filesystem permissions (default 0600)
