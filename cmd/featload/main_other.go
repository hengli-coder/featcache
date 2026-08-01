//go:build !linux

package main

import (
	"flag"
	"fmt"
	"log"
)

func main() {
	flag.Parse()
	log.Fatal("featload requires Linux (shared memory not supported on this platform)")
}

// keep unused import
var _ = fmt.Sprintf
