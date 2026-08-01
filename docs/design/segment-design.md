# Segment Design

## 1. Problem

The Loader needs to write large amounts of read-only data (GB ~ TB scale) into memory and let multiple inference processes share it zero-copy. Core requirements:

- Processes share the same physical memory
- Lock-free, syscall-free reads
- Lifecycle management (create/open/close/destroy)

## 2. Goals

- Support segments of 10GB+ of data
- Reader open < 100ms
- Cross-platform abstraction (real Linux implementation, testable stubs elsewhere)

## 3. Non-Goals

- Cross-machine sharing (distributed)
- Dynamic resizing (fixed segment size)
- Data persistence to disk (Phase 3)

## 4. Proposed Design

### 4.1 Platform abstraction

```go
// Segment is a handle to a shared memory segment
type Segment struct {
    name string
    data []byte
    cap  int
}

// Platform-specific implementation (build tags)
CreateSegment(name, size) → segment  // shm_open + mmap
OpenSegment(name)        → segment  // open an existing segment
(s *Segment) close()     → error    // munmap
(s *Segment) destroy()   → error    // munmap + unlink
```

| Platform | Implementation |
|----------|----------------|
| Linux | `/dev/shm/<name>` + `unix.Mmap(MAP_SHARED)` |
| Other | `ErrNotSupported` stubs (in-memory close/destroy support tests) |

### 4.2 Linux implementation details

```go
// Create (O_EXCL prevents clobbering an existing segment)
fd, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
fd.Truncate(size)                       // set the segment size
data, err := unix.Mmap(fd, 0, size, PROT_READ|PROT_WRITE, MAP_SHARED)
```

- File permissions 0600: same-user access only
- On create failure (already exists), fall back to OpenSegment
- `close()` only unmaps; `destroy()` additionally removes the file

### 4.3 In-memory segments (test support)

Non-Linux platforms do not provide real shared memory, but a `Segment` can be constructed directly from `[]byte` to test the full Loader→Reader path:

```go
seg := &Segment{name: "test", data: make([]byte, size), cap: size}
```

## 5. Alternatives Considered

| Option | Pros | Cons | Conclusion |
|--------|------|------|------------|
| POSIX shm (chosen) | memory-backed, clear sharing semantics | Linux only | ✅ |
| File mmap | persistable | disk I/O, complex lifecycle | ❌ |
| SysV IPC shm | old API | deprecation trend, cumbersome interface | ❌ |

## 6. Risks

| Risk | Mitigation |
|------|------------|
| `/dev/shm` capacity limits | documented; segment size configurable |
| Segment residue (process crash) | explicit `destroy()`; `O_EXCL` prevents accidental clobbering |
| Insufficient permissions | 0600 permissions + same-user deployment |

## 7. Related Code

- `pkg/featcache/segment.go` (platform-independent interface)
- `pkg/featcache/segment_linux.go` (Linux implementation)
- `pkg/featcache/segment_other.go` (non-Linux stubs)
