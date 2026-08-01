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

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hengli-coder/featcache/pkg/featcache"
)

// version information, set at build time via -ldflags (see .goreleaser.yml).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	segmentName := flag.String("name", "featcache", "shared memory segment name")
	cacheSize := flag.Int("size", 2<<30, "shared memory segment size in bytes (default 2GB)")
	udsPath := flag.String("uds", "\x00featcache", "UDS abstract socket path (prefix with \\x00 for abstract namespace)")
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	if *showVersion {
		log.Printf("featload %s (commit %s, built %s)", version, commit, date)
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

	server, err := featcache.NewCacheServer(*segmentName, *cacheSize, udsAddr)
	if err != nil {
		log.Fatalf("failed to create cache server: %v", err)
	}

	// Handle shutdown signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("received signal %v, shutting down", sig)
		server.Close()
		os.Exit(0)
	}()

	log.Printf("listening on %s", udsAddr)
	if err := server.Listen(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
