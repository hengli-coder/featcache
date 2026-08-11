//go:build !linux

package featcache

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/hengli-coder/featcache/pkg/shm"
)

// TestReaderViaUDS_Discovery verifies that a Reader can connect to a loader's
// control-plane socket, discover the shared memory segment name/layout via
// GET_INFO, then read data zero-copy — without being told the segment name.
//
// This runs on any OS using in-memory segments + a filesystem UDS socket
// (a short path under /tmp to stay within macOS's ~103-byte UDS path limit).
func TestReaderViaUDS_Discovery(t *testing.T) {
	addr := "/tmp/ftc-disc-" + time.Now().Format("150405")
	os.Remove(addr)

	name := "discovered-seg"
	seg := shm.NewInMemorySegment(name, 64*1024+1024*1024)
	// Register the in-memory segment so the Reader can re-open it by name
	// across the UDS boundary (the non-Linux analogue of /dev/shm lookup).
	shm.RegisterTestSegment(name, seg)
	defer shm.UnregisterTestSegment(name)

	// Load data into the segment.
	loader, err := newLoaderWithSegment(LoaderConfig{SegmentName: name}, seg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loader.Load(NewMapDataSource(map[string][]byte{
		"user:789": []byte("discovered_emb"),
		"token:词表": []byte("中文token"),
	})); err != nil {
		t.Fatal(err)
	}

	// Start a loader control-plane server over a filesystem UDS socket.
	srv := NewServer(seg, addr)
	srv.SetState(StateReady)
	go func() { _ = srv.Listen() }()

	// Wait for the listener to come up.
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err := net.Dial("unix", addr)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not come up in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
	defer func() {
		srv.Close()
		os.Remove(addr)
	}()

	// Reader that discovers the segment via UDS only — no segment name supplied.
	reader, err := NewReaderViaUDS(addr)
	if err != nil {
		t.Fatalf("NewReaderViaUDS: %v", err)
	}
	defer reader.Close()

	// The reader must have mapped the correct segment.
	if got := reader.Segment().Name(); got != name {
		t.Fatalf("Segment().Name() = %q, want %q", got, name)
	}

	// Zero-copy lookups against the discovered segment.
	for k, v := range map[string]string{
		"user:789": "discovered_emb",
		"token:词表": "中文token",
	} {
		got, ok := reader.Get([]byte(k))
		if !ok {
			t.Fatalf("Get(%q) = miss, want hit", k)
		}
		if string(got) != v {
			t.Fatalf("Get(%q) = %q, want %q", k, got, v)
		}
	}
	if _, ok := reader.Get([]byte("missing")); ok {
		t.Fatal("Get(missing) = hit, want miss")
	}
}

// TestNewReaderViaUDS_NameMismatch verifies that when a reader is given an
// explicit segment name that disagrees with the loader, it fails rather than
// silently mapping the wrong segment.
func TestNewReaderViaUDS_NameMismatch(t *testing.T) {
	addr := "/tmp/ftc-mismatch-" + time.Now().Format("150405")
	os.Remove(addr)

	seg := shm.NewInMemorySegment("real-name", 64*1024+1024*1024)
	srv := NewServer(seg, addr)
	srv.SetState(StateReady)
	go func() { _ = srv.Listen() }()

	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err := net.Dial("unix", addr)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not come up in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
	defer func() {
		srv.Close()
		os.Remove(addr)
	}()

	// Provide a conflicting segment name — must fail.
	if _, err := NewReader("wrong-name", addr); err == nil {
		t.Fatal("expected error on segment name mismatch")
	}
}
