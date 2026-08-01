package featcache

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"testing"
)

// --- CacheServer tests (filesystem UDS path; runs on any OS) ---

// startTestServer starts a CacheServer listening on a temp UDS path.
// Uses a short path name to stay within macOS 103-byte UDS path limit.
func startTestServer(t *testing.T, name, testID string) (srv *CacheServer, cleanup func()) {
	t.Helper()
	// macOS has a ~103-byte limit on Unix socket paths, so we use a short
	// path under /tmp rather than t.TempDir() which can produce paths >100 bytes.
	addr := "/tmp/ftc-" + testID
	os.Remove(addr)

	seg := &Segment{
		name: name,
		data: make([]byte, 64*1024+1024*1024),
		cap:  64*1024 + 1024*1024,
	}
	s := &CacheServer{
		segmentName: name,
		segmentSize: seg.cap,
		udsAddr:     addr,
		seg:         seg,
	}

	// Create the listener synchronously before starting the accept goroutine.
	unixAddr := &net.UnixAddr{Name: addr, Net: "unix"}
	ln, err := net.ListenUnix("unix", unixAddr)
	if err != nil {
		t.Fatalf("ListenUnix(%q): %v", addr, err)
	}
	s.ln = ln

	// Accept loop in the background.
	go func() {
		for !s.closed.Load() {
			conn, err := ln.AcceptUnix()
			if err != nil {
				if s.closed.Load() {
					break
				}
				continue
			}
			go s.handleConn(conn)
		}
	}()

	cleanup = func() {
		_ = s.Close()
		os.Remove(addr)
	}
	return s, cleanup
}

// dialAndRequest sends a request and returns the decoded response.
func dialAndRequest(t *testing.T, addr string, req *Request) *Response {
	t.Helper()
	conn, err := net.Dial("unix", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := EncodeRequest(conn, req); err != nil {
		t.Fatalf("encode request: %v", err)
	}
	resp, err := DecodeResponse(conn)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestCacheServerGetInfo(t *testing.T) {
	s, cleanup := startTestServer(t, "test-seg", "gi")
	defer cleanup()

	// Initialize the segment header so GetInfo returns meaningful metadata.
	_ = s.seg.Header() // Initialize the underlying segment.
	hdr := s.seg.Header()
	hdr.Size = uint64(s.seg.cap)
	hdr.HashOffset = HeaderSize
	hdr.HashCap = 64
	hdr.DataOffset = Align(HeaderSize+64*SlotSize, 8)

	resp := dialAndRequest(t, "/tmp/ftc-gi", &Request{Op: OpGetInfo})
	if resp.Status != RespOK {
		t.Fatalf("status = %d, want RespOK", resp.Status)
	}
	if resp.SegmentName != "test-seg" {
		t.Fatalf("SegmentName = %q, want %q", resp.SegmentName, "test-seg")
	}
	if resp.DataOffset <= resp.HashOffset {
		t.Fatalf("DataOffset %d must be after HashOffset %d", resp.DataOffset, resp.HashOffset)
	}
	if resp.DataOffset < HeaderSize+64*SlotSize {
		t.Fatalf("DataOffset %d too small", resp.DataOffset)
	}
}

func TestCacheServerGetStatus(t *testing.T) {
	_, cleanup := startTestServer(t, "test-seg", "gs")
	defer cleanup()

	resp := dialAndRequest(t, "/tmp/ftc-gs", &Request{Op: OpGetStatus})
	if resp.Status != RespOK {
		t.Fatalf("status = %d, want RespOK", resp.Status)
	}
}

func TestCacheServerUnknownOp(t *testing.T) {
	_, cleanup := startTestServer(t, "test-seg", "uo")
	defer cleanup()

	resp := dialAndRequest(t, "/tmp/ftc-uo", &Request{Op: OpCode(0xFF)})
	if resp.Status != RespError {
		t.Fatalf("status = %d, want RespError", resp.Status)
	}
}

func TestCacheServerCloseAndDestroy(t *testing.T) {
	// Close (not destroy) leaves the backing file in place.
	s := &CacheServer{
		segmentName: "close-test",
		segmentSize: 64*1024 + 1024*1024,
		udsAddr:     "", // no UDS path → Close skips unlink
		seg:         &Segment{name: "close-test", data: make([]byte, 64*1024+1024*1024), cap: 64*1024 + 1024*1024},
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Destroy on a fresh server.
	s2 := &CacheServer{
		segmentName: "destroy-test",
		segmentSize: 64*1024 + 1024*1024,
		udsAddr:     "",
		seg:         &Segment{name: "destroy-test", data: make([]byte, 64*1024+1024*1024), cap: 64*1024 + 1024*1024},
	}
	if err := s2.Destroy(); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
}

// TestCacheServerConcurrentClients verifies the server handles multiple
// simultaneous control-plane connections.
func TestCacheServerConcurrentClients(t *testing.T) {
	_, cleanup := startTestServer(t, "test-seg", "cc")
	defer cleanup()

	const n = 8
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			resp := dialAndRequest(t, "/tmp/ftc-cc", &Request{Op: OpGetInfo})
			if resp.Status != RespOK {
				errCh <- errors.New("bad status from concurrent client")
				return
			}
			errCh <- nil
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
}

// --- Segment header helpers ---

func TestSegmentHeaderAccessors(t *testing.T) {
	seg := newTestSegment(t, 0)
	hdr := seg.Header()
	hdr.Magic = Magic
	hdr.HashOffset = 64
	hdr.DataOffset = 4096
	hdr.HashCap = 256
	hdr.GenCounter = 7

	if seg.HashOffset() != 64 {
		t.Fatalf("HashOffset() = %d", seg.HashOffset())
	}
	if seg.DataOffset() != 4096 {
		t.Fatalf("DataOffset() = %d", seg.DataOffset())
	}
	if seg.HashCap() != 256 {
		t.Fatalf("HashCap() = %d", seg.HashCap())
	}
	if seg.GenCounter() != 7 {
		t.Fatalf("GenCounter() = %d", seg.GenCounter())
	}
	if seg.Cap() <= 0 {
		t.Fatal("Cap() must be positive")
	}
	if seg.Name() != "test-segment" {
		t.Fatalf("Name() = %q", seg.Name())
	}
}

// TestHashSlotSizeAligned ensures the slot struct stays 24 bytes with the
// expected field layout so the unsafe overlay and binary layout agree.
func TestHashSlotSizeAligned(t *testing.T) {
	var s HashSlot
	_ = binary.Size(s) // compile-time check: binary serializable
	if got := binary.Size(s); got != SlotSize {
		t.Fatalf("binary size of HashSlot = %d, want %d", got, SlotSize)
	}
}

func TestHashTableIterateAndSlotAt(t *testing.T) {
	buf := make([]byte, 64*1024+1024*1024)
	const dataBase = 64 * 1024
	ht := NewHashTable(buf, 0, dataBase, 64)
	InitHashTable(buf, 0, 64)

	keys := [][]byte{[]byte("k1"), []byte("k2"), []byte("k3")}
	dataOff := uint32(0)
	for _, k := range keys {
		v := []byte("v")
		binary.LittleEndian.PutUint32(buf[dataBase+int(dataOff):dataBase+int(dataOff)+4], uint32(len(k)))
		copy(buf[dataBase+int(dataOff)+4:], k)
		copy(buf[dataBase+int(dataOff)+4+len(k):], v)
		if !ht.Insert(HashKey(k), k, dataOff, uint32(len(v))) {
			t.Fatalf("insert %q failed", k)
		}
		dataOff += uint32(4 + len(k) + len(v))
	}

	if ht.Count() != 3 {
		t.Fatalf("Count() = %d, want 3", ht.Count())
	}

	var iterated []string
	ht.Iterate(func(_ HashSlot) bool {
		iterated = append(iterated, "slot")
		return true
	})
	if len(iterated) != 3 {
		t.Fatalf("Iterate visited %d slots, want 3", len(iterated))
	}

	// Early-stop iteration.
	var stopped []string
	ht.Iterate(func(_ HashSlot) bool {
		stopped = append(stopped, "x")
		return false
	})
	if len(stopped) != 1 {
		t.Fatalf("early-stop Iterate visited %d slots, want 1", len(stopped))
	}

	// SlotAt returns used slots with matching hash.
	for i := 0; i < ht.capacity; i++ {
		slot := ht.SlotAt(i)
		if slot.Status == SlotUsed {
			if slot.Hash == 0 {
				t.Fatal("used slot has zero hash")
			}
		}
	}
}
