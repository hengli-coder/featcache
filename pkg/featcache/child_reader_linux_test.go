//go:build linux

package featcache

import (
	"encoding/binary"
	"flag"
	"os"
	"os/exec"
	"testing"
)

// buildHelperCommand constructs a re-exec of the current test binary that
// runs the child-reader helper test. Flags carry the segment name and a
// request file path.
func buildHelperCommand(args []string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = os.Environ()
	// The child re-exec runs `go test -run TestE2EChildReader`. The default
	// `-test.testlogfile`/`-test.paniconexit0` flags make failures surface as a
	// non-zero exit; capturing stderr lets the parent report why it failed.
	if testing.Verbose() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd
}

// childRunner parses the -ftc.segment and -ftc.request flags registered only
// in the helper test binary.
var (
	ftcSegment   = flag.String("ftc.segment", "", "shared memory segment name (child reader)")
	ftcRequest   = flag.String("ftc.request", "", "path to request file (child reader)")
	childDefined bool
)

// TestE2EChildReader is a helper process. It is spawned by forkReaderVerifier
// and opens the named segment, reads the keys from the request file, and
// fails the test (non-zero exit via t.Fatal propagation to the test framework)
// if any key is missing.
//
// It only runs when -ftc.segment is set, which happens exclusively from the
// forkReaderVerifier parent, so it is dormant during normal `go test`.
func TestE2EChildReader(t *testing.T) {
	if *ftcSegment == "" {
		t.Skip("not a child reader invocation")
	}

	// Read the request file: [count:4][len:4][key...]...
	req, err := os.ReadFile(*ftcRequest)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	n := int(binary.BigEndian.Uint32(req[:4]))
	off := 4
	keys := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		klen := int(binary.BigEndian.Uint32(req[off : off+4]))
		off += 4
		keys = append(keys, req[off:off+klen])
		off += klen
	}

	// Open the segment by name (a real cross-process open of /dev/shm/<name>).
	seg, err := OpenSegment(*ftcSegment)
	if err != nil {
		t.Fatalf("open segment: %v", err)
	}
	defer seg.Close()

	r, err := NewReaderFromSegment(seg)
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	defer r.Close()

	for _, k := range keys {
		if _, ok := r.Get(k); !ok {
			t.Fatalf("key %q not found in child", k)
		}
	}
}
