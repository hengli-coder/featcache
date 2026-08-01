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
	"encoding/binary"
	"io"
)

// Binary protocol for UDS communication (control plane only).
//
// Request format:
//
//	OpCode:   1B
//	KeyLen:   2B (uint16, big-endian)
//	Key:      KeyLen bytes
//
// Response format:
//
//	StatusCode:   1B
//	SegmentName: 64B (fixed, null-padded)
//	SegmentSize: 8B (uint64, big-endian)
//	HashOffset:  4B (uint32, big-endian)
//	HashCap:     4B (uint32, big-endian)
//	DataOffset:  4B (uint32, big-endian)
//	GenCounter:  8B (uint64, big-endian)

const (
	// ReqHeaderLen is the request header size.
	ReqHeaderLen = 3

	// RespHeaderLen is the response header size.
	RespHeaderLen = 1 + 64 + 8 + 4 + 4 + 4 + 8 // 93 bytes

	// MaxMsgLen is the maximum total message size.
	MaxMsgLen = 1 << 20 // 1 MB
)

// Request represents a parsed client request.
type Request struct {
	Op  OpCode
	Key []byte
}

// Response represents a server response.
type Response struct {
	Status      StatusCode
	SegmentName string
	SegmentSize uint64
	HashOffset  uint32
	HashCap     uint32
	DataOffset  uint32
	GenCounter  uint64
}

// EncodeRequest writes a request to w.
func EncodeRequest(w io.Writer, req *Request) error {
	keyLen := len(req.Key)
	if keyLen > 65535 {
		keyLen = 65535
	}

	header := make([]byte, ReqHeaderLen)
	header[0] = byte(req.Op)
	binary.BigEndian.PutUint16(header[1:3], uint16(keyLen))

	if _, err := w.Write(header); err != nil {
		return err
	}
	if keyLen > 0 {
		if _, err := w.Write(req.Key[:keyLen]); err != nil {
			return err
		}
	}
	return nil
}

// DecodeRequest reads and parses a request from r.
func DecodeRequest(r io.Reader) (*Request, error) {
	header := make([]byte, ReqHeaderLen)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	keyLen := int(binary.BigEndian.Uint16(header[1:3]))
	if keyLen > MaxMsgLen {
		return nil, io.ErrUnexpectedEOF
	}

	req := &Request{
		Op: OpCode(header[0]),
	}

	if keyLen > 0 {
		req.Key = make([]byte, keyLen)
		if _, err := io.ReadFull(r, req.Key); err != nil {
			return nil, err
		}
	}

	return req, nil
}

// EncodeResponse writes a response to w.
func EncodeResponse(w io.Writer, resp *Response) error {
	header := make([]byte, RespHeaderLen)
	off := 0

	header[off] = byte(resp.Status)
	off++

	// SegmentName: 64B, null-padded
	name := resp.SegmentName
	if len(name) > 64 {
		name = name[:64]
	}
	copy(header[off:], name)
	off += 64

	binary.BigEndian.PutUint64(header[off:off+8], resp.SegmentSize)
	off += 8
	binary.BigEndian.PutUint32(header[off:off+4], resp.HashOffset)
	off += 4
	binary.BigEndian.PutUint32(header[off:off+4], resp.HashCap)
	off += 4
	binary.BigEndian.PutUint32(header[off:off+4], resp.DataOffset)
	off += 4
	binary.BigEndian.PutUint64(header[off:off+8], resp.GenCounter)

	_, err := w.Write(header)
	return err
}

// DecodeResponse reads and parses a response from r.
func DecodeResponse(r io.Reader) (*Response, error) {
	header := make([]byte, RespHeaderLen)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	off := 0
	resp := &Response{
		Status: StatusCode(header[off]),
	}
	off++

	resp.SegmentName = string(header[off : off+64])
	// Trim trailing null bytes
	for i := len(resp.SegmentName) - 1; i >= 0; i-- {
		if resp.SegmentName[i] != 0 {
			resp.SegmentName = resp.SegmentName[:i+1]
			break
		}
	}
	off += 64

	resp.SegmentSize = binary.BigEndian.Uint64(header[off : off+8])
	off += 8
	resp.HashOffset = binary.BigEndian.Uint32(header[off : off+4])
	off += 4
	resp.HashCap = binary.BigEndian.Uint32(header[off : off+4])
	off += 4
	resp.DataOffset = binary.BigEndian.Uint32(header[off : off+4])
	off += 4
	resp.GenCounter = binary.BigEndian.Uint64(header[off : off+8])

	return resp, nil
}
