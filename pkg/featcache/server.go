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

package featcache

import (
	"log"
	"net"
	"os"
	"sync/atomic"
)

// ServerState enumerates the loader lifecycle states, surfaced via GET_STATUS.
type ServerState int32

const (
	StateIdle     ServerState = iota // 0 — before loading
	StateLoading                     // 1 — loading data
	StateReady                       // 2 — data ready, readers may use it
	StateUpdating                    // 3 — reloading (Phase 2)
)

// CacheServer is the UDS control-plane server for featcache.
// It serves metadata requests from Reader processes over a Unix Domain Socket.
// The actual data is stored in shared memory and read directly by Readers.
type CacheServer struct {
	segmentName string
	segmentSize int
	udsAddr     string

	seg *Segment

	ln     net.Listener
	closed atomic.Bool

	// state is the current loader state, served via GET_STATUS.
	state atomic.Int32
}

// NewCacheServer creates a CacheServer that manages a shared memory segment
// and serves control-plane requests over UDS.
func NewCacheServer(segmentName string, segmentSize int, udsAddr string) (*CacheServer, error) {
	seg, err := CreateSegment(segmentName, segmentSize)
	if err != nil {
		seg, err = OpenSegment(segmentName)
		if err != nil {
			return nil, err
		}
	}

	s := &CacheServer{
		segmentName: segmentName,
		segmentSize: segmentSize,
		udsAddr:     udsAddr,
		seg:         seg,
	}
	return s, nil
}

// NewServer creates a CacheServer over an already-open segment.
// Used by tests and by the loader daemon when it owns the segment and wants
// to expose it over UDS without re-opening it.
func NewServer(seg *Segment, udsAddr string) *CacheServer {
	// The server shares ownership of the segment (we map the same memory, not
	// a new mapping), so Destroy must not unlink a backing file the caller may
	// not have. Mark it as not file-backed so Destroy is a no-op for the file.
	seg.backedByFile = false
	return &CacheServer{
		segmentName: seg.Name(),
		segmentSize: seg.Cap(),
		udsAddr:     udsAddr,
		seg:         seg,
	}
}

// SetState updates the loader state.
func (s *CacheServer) SetState(state ServerState) {
	s.state.Store(int32(state))
}

// State returns the current loader state.
func (s *CacheServer) State() ServerState {
	return ServerState(s.state.Load())
}

// Segment returns the underlying shared memory segment.
func (s *CacheServer) Segment() *Segment {
	return s.seg
}

// Listen starts the UDS listener and serves control-plane requests.
func (s *CacheServer) Listen() error {
	if s.udsAddr != "" && s.udsAddr[0] == '/' {
		os.Remove(s.udsAddr)
	}

	addr := &net.UnixAddr{Name: s.udsAddr, Net: "unix"}
	ln, err := net.ListenUnix("unix", addr)
	if err != nil {
		return err
	}
	if s.udsAddr != "" && s.udsAddr[0] == '/' {
		_ = os.Chmod(s.udsAddr, 0o777)
	}
	s.ln = ln

	log.Printf("featcache: listening on %s", s.udsAddr)

	for !s.closed.Load() {
		conn, err := ln.AcceptUnix()
		if err != nil {
			if s.closed.Load() {
				break
			}
			log.Printf("featcache: accept error: %v", err)
			continue
		}
		go s.handleConn(conn)
	}
	return nil
}

// Close shuts down the server.
func (s *CacheServer) Close() error {
	s.closed.Store(true)
	if s.ln != nil {
		s.ln.Close()
	}
	if s.udsAddr != "" && s.udsAddr[0] == '/' {
		os.Remove(s.udsAddr)
	}
	return s.seg.Close()
}

// Destroy closes the server and destroys the shared memory segment.
func (s *CacheServer) Destroy() error {
	s.closed.Store(true)
	if s.ln != nil {
		s.ln.Close()
	}
	if s.udsAddr != "" && s.udsAddr[0] == '/' {
		os.Remove(s.udsAddr)
	}
	return s.seg.Destroy()
}

func (s *CacheServer) handleConn(conn *net.UnixConn) {
	defer conn.Close()

	req, err := DecodeRequest(conn)
	if err != nil {
		return
	}

	var resp Response
	switch req.Op {
	case OpGetInfo:
		resp = s.handleGetInfo()
	case OpGetStatus:
		resp = s.handleGetStatus()
	default:
		resp = Response{Status: RespError}
	}

	_ = EncodeResponse(conn, &resp)
}

func (s *CacheServer) handleGetInfo() Response {
	hdr := s.seg.Header()
	return Response{
		Status:      RespOK,
		SegmentName: s.segmentName,
		SegmentSize: hdr.Size,
		HashOffset:  hdr.HashOffset,
		HashCap:     hdr.HashCap,
		DataOffset:  hdr.DataOffset,
		GenCounter:  hdr.GenCounter,
	}
}

func (s *CacheServer) handleGetStatus() Response {
	return Response{
		Status: RespOK,
	}
}
