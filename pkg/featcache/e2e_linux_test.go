//go:build linux

// Linux-only end-to-end tests that exercise the full Loader → Server → Reader
// flow over a real POSIX shared memory segment (/dev/shm). On non-Linux
// platforms these run nothing (the real shm path requires Linux mmap).

package featcache

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestE2ERealShmLoadAndRead(t *testing.T) {
	name := fmt.Sprintf("ftc-e2e-%d", os.Getpid())
	shmPath := devShmPath(name)
	os.Remove(shmPath) // clean any stale segment from a previous crash
	defer os.Remove(shmPath)

	// 1. Loader creates and writes a real shared memory segment.
	loader, err := NewLoader(LoaderConfig{
		SegmentName: name,
		SegmentSize: 64*1024 + 1024*1024, // ~1MB
	})
	if err != nil {
		t.Fatal(err)
	}
	defer loader.Destroy()

	entries := NewMapDataSource(map[string][]byte{
		"user:123":   []byte("embedding_123"),
		"item:456:1": []byte("emb_456"),
		"vocab:中国":   []byte("中文字符"),
	})
	if _, err := loader.Load(entries); err != nil {
		t.Fatal(err)
	}

	// Verify a second process (simulated by opening via name) sees the data.
	// Here we spawn a separate OS process so the mapping is genuinely cross-process.
	proc, err := forkReaderVerifier(name, []string{"user:123", "item:456:1", "vocab:中国"})
	if err != nil {
		t.Fatalf("spawn child: %v", err)
	}
	status := waitChild(proc)
	if status != 0 {
		t.Fatalf("child reader returned %d", status)
	}
}

func TestE2EServerGetInfoOverRealShm(t *testing.T) {
	name := fmt.Sprintf("ftc-srv-%d", os.Getpid())
	shmPath := devShmPath(name)
	os.Remove(shmPath)
	defer os.Remove(shmPath)

	// Build a segment with the loader.
	loader, err := NewLoader(LoaderConfig{SegmentName: name, SegmentSize: 64*1024 + 1024*1024})
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.Init(8); err != nil {
		t.Fatal(err)
	}
	_, _ = loader.Load(NewMapDataSource(map[string][]byte{"a": []byte("b")}))

	// Start a UDS server on an abstract address.
	udsAddr := "\x00ftc-e2e-srv"
	srv := NewServer(loader.Segment(), udsAddr)
	srv.SetState(StateReady)
	go func() {
		_ = srv.Listen()
	}()
	defer srv.Close()

	// Wait for listener to be up.
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err := net.Dial("unix", udsAddr)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not come up in time")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Send GET_INFO and verify metadata.
	conn, err := net.Dial("unix", udsAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := EncodeRequest(conn, &Request{Op: OpGetInfo}); err != nil {
		t.Fatal(err)
	}
	resp, err := DecodeResponse(conn)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != RespOK {
		t.Fatalf("status = %d", resp.Status)
	}
	if resp.SegmentName != name {
		t.Fatalf("SegmentName = %q, want %q", resp.SegmentName, name)
	}
	if resp.HashOffset < HeaderSize {
		t.Fatalf("HashOffset = %d", resp.HashOffset)
	}
	if resp.DataOffset <= resp.HashOffset {
		t.Fatalf("DataOffset = %d not after HashOffset %d", resp.DataOffset, resp.HashOffset)
	}
}

func TestE2ESegmentLayoutConsistency(t *testing.T) {
	name := fmt.Sprintf("ftc-layout-%d", os.Getpid())
	shmPath := devShmPath(name)
	os.Remove(shmPath)
	defer os.Remove(shmPath)

	loader, err := NewLoader(LoaderConfig{SegmentName: name, SegmentSize: 128 * 1024})
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.Init(100); err != nil {
		t.Fatal(err)
	}

	hdr := loader.Segment().Header()
	if int(hdr.DataOffset) >= int(hdr.Size) {
		t.Fatalf("DataOffset %d exceeds size %d", hdr.DataOffset, hdr.Size)
	}
	// Data region starts at DataOffset; hash table is between HashOffset and DataOffset.
	if int(hdr.HashOffset)+int(hdr.HashCap)*SlotSize > int(hdr.DataOffset) {
		t.Fatalf("hash table overlaps data region")
	}
	// First data bytes are initialized (zeroed hash table, empty data).
	if hdr.DataEnd != hdr.DataOffset {
		t.Fatalf("DataEnd = %d, want %d", hdr.DataEnd, hdr.DataOffset)
	}
}

// --- Cross-process reader verifier ---
//
// forkReaderVerifier spawns a child process that opens the named segment,
// reads specific keys, and exits 0 if all are present, else exits non-zero.

func forkReaderVerifier(segmentName string, keys []string) (*os.Process, error) {
	// Write a small request file the child reads keys from and reports results.
	dataDir, _ := os.MkdirTemp("", "ftc-verify")
	requestPath := filepath.Join(dataDir, "keys")

	// Encode: count, then per key (len + bytes)
	var req []byte
	req = binary.BigEndian.AppendUint32(req, uint32(len(keys)))
	for _, k := range keys {
		req = binary.BigEndian.AppendUint32(req, uint32(len(k)))
		req = append(req, k...)
	}
	if err := os.WriteFile(requestPath, req, 0o644); err != nil {
		return nil, err
	}

	// Re-exec this test binary as a helper with a special -test.run flag.
	args := []string{
		"-test.run", "TestE2EChildReader",
		"-ftc.segment", segmentName,
		"-ftc.request", requestPath,
	}
	cmd := buildHelperCommand(args)
	return cmd.Process, cmd.Start()
}

func waitChild(proc *os.Process) int {
	state, err := proc.Wait()
	if err != nil {
		return -1
	}
	return state.ExitCode()
}
