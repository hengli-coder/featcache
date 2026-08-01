# Control-Plane Protocol Design

## 1. Problem

Clients need to fetch shared memory segment metadata (name, size, layout) during initialization. This requires a lightweight, reliable inter-process communication protocol.

## 2. Goals

- Support `OpGetInfo` (metadata) and `OpGetStatus` (status)
- Lightweight: connect-and-use, no handshake
- Extensible: reserve WATCH/PIN/PREFETCH/EVICT/LIST/RELOAD for Phases 2–3

## 3. Non-Goals

- Data transfer (all data goes through shared memory)
- Encryption (local IPC, same user)
- Long-lived connections (disconnect after initialization)

## 4. Proposed Design

### 4.1 Transport

Unix Domain Socket:

- Abstract namespace (`\x00` prefix, no filesystem entry)
- Or filesystem path (`/` prefix)

### 4.2 Request

```
OpCode: 1B | KeyLen: 2B (BE) | Key: KeyLen B
```

### 4.3 Response

```
Status: 1B | SegmentName: 64B | SegmentSize: 8B (BE)
| HashOffset: 4B (BE) | HashCap: 4B (BE) | DataOffset: 4B (BE) | GenCounter: 8B (BE)
```

### 4.4 Server

```go
// CacheServer: UDS control-plane server
type CacheServer struct {
    segmentName string
    segmentSize int
    udsAddr     string
    seg         *Segment
    ln          net.Listener
    closed      atomic.Bool
}

// One goroutine per connection
handleConn:
  req := DecodeRequest(conn)
  switch req.Op {
    case OpGetInfo:   resp = handleGetInfo()   // read the layout from the Header
    case OpGetStatus: resp = handleGetStatus()
    default:          resp = Response{Status: RespError}
  }
  EncodeResponse(conn, resp)
```

## 5. Alternatives Considered

| Option | Pros | Cons | Conclusion |
|--------|------|------|------------|
| UDS custom binary (chosen) | lightweight, low latency | must write own codec | ✅ |
| HTTP/gRPC | mature ecosystem | heavyweight, high latency | ❌ |
| Known conventions only (no control plane) | simplest | clients need external layout config | ❌ |

## 6. Risks

| Risk | Mitigation |
|------|------------|
| Protocol mismatch (version drift) | `Version` field in the Header |
| Long messages (KeyLen ceiling) | `MaxMsgLen` 1MB validation |
| UDS path length (macOS 103B) | tests use short paths; production Linux abstract namespace has no such limit |
| Concurrent connections | one goroutine per connection |

## 7. Related Code

- `pkg/featcache/protocol.go`
- `pkg/featcache/server.go`
- Tests: `pkg/featcache/server_test.go`
