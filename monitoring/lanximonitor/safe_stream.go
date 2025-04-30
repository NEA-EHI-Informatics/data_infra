// safe_stream.go
package main

import (
	"fmt"
	"io"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
)

const maxAllowedRead = 10 * 1024 * 1024 // 10MB

type SafeStream struct {
	*kaitai.Stream
	brs *bufferedReadSeeker
}

func NewSafeStream(r io.Reader) *SafeStream {
	brs := newBufferedReadSeeker(r)
	return &SafeStream{
		Stream: kaitai.NewStream(brs),
		brs:    brs,
	}
}

func (s *SafeStream) ReadBytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("invalid read length: %d", n)
	}

	if n > maxAllowedRead {
		// Important: Discard data to maintain stream alignment
		discard := make([]byte, n)
		_, err := io.ReadFull(s.Stream, discard)
		if err != nil {
			return nil, fmt.Errorf("oversized read (%d bytes) failed: %w", n, err)
		}
		return nil, fmt.Errorf("read size %d exceeds limit %d", n, maxAllowedRead)
	}

	return s.Stream.ReadBytes(n)
}

// Helper method to reset the stream on reconnection
func (s *SafeStream) Reset(r io.Reader) {
	s.brs = newBufferedReadSeeker(r)
	s.Stream = kaitai.NewStream(s.brs)
}
