//go:build linux

// Copyright 2026 featcache contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command featload is the featcache loader daemon.
//
// It loads key-value entries from a data source into a POSIX shared memory
// segment, then serves the segment metadata over a Unix Domain Socket so that
// inference processes can mmap and read the data zero-copy.
//
// Usage:
//
//	featload -name my-embeddings -size 10737418240 -source /data/embeddings.tsv -uds "\x00featcache-my-embeddings"
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hengli-coder/featcache/pkg/featcache"
)

// version information, set at build time via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	segmentName := flag.String("name", "featcache", "shared memory segment name")
	cacheSize := flag.Int("size", 2<<30, "shared memory segment size in bytes (default 2GB)")
	udsPath := flag.String("uds", "\x00featcache", "UDS abstract socket path (prefix with \\x00 for abstract namespace)")
	sourcePath := flag.String("source", "", "path to data source file (tab-separated lines: key<TAB>value). "+
		"If empty, the segment is created empty and only metadata is served.")
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("featload %s (commit %s, built %s)\n", version, commit, date)
		return
	}

	// Convert \x00 prefix string to actual null byte for abstract sockets.
	udsAddr := *udsPath
	if strings.HasPrefix(udsAddr, "\\x00") {
		udsAddr = "\x00" + udsAddr[4:]
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("featload %s starting", version)
	log.Printf("  segment: %s (%d MB)", *segmentName, *cacheSize>>20)
	log.Printf("  uds:     %s", udsAddr)
	if *sourcePath != "" {
		log.Printf("  source:  %s", *sourcePath)
	}

	// ─── Loader: build the shared memory segment ───────────────────────
	loader, err := featcache.NewLoader(featcache.LoaderConfig{
		SegmentName: *segmentName,
		SegmentSize: *cacheSize,
	})
	if err != nil {
		log.Fatalf("create loader: %v", err)
	}

	if *sourcePath != "" {
		ds := featcache.NewLineDataSource(*sourcePath)
		if _, err := loader.Load(ds); err != nil {
			log.Fatalf("load data source: %v", err)
		}
	} else {
		// No source: create an initialized (empty) segment.
		if err := loader.Init(0); err != nil {
			log.Fatalf("init empty segment: %v", err)
		}
	}

	// ─── Server: serve the segment metadata over UDS ───────────────────
	server := featcache.NewServer(loader.Segment(), udsAddr)
	server.SetState(featcache.StateReady)

	// Handle shutdown signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received signal %v, shutting down", sig)
		_ = loader.Close()
		os.Exit(0)
	}()

	log.Printf("listening on %s", udsAddr)
	if err := server.Listen(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
