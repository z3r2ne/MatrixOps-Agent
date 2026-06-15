package streamtypes

import (
	"bytes"
	"io"
	"sync"
	"sync/atomic"
)

// StreamingActionBuffer is an incrementally writable, blockingly readable
// in-memory buffer used for native streaming tool args / answer text.
type StreamingActionBuffer struct {
	id     uint64
	mu     sync.Mutex
	cond   *sync.Cond
	buf    bytes.Buffer
	closed bool
	err    error
}

var streamingActionBufferSeq uint64

// NewStreamingActionBuffer creates a new StreamingActionBuffer.
func NewStreamingActionBuffer() *StreamingActionBuffer {
	out := &StreamingActionBuffer{id: atomic.AddUint64(&streamingActionBufferSeq, 1)}
	out.cond = sync.NewCond(&out.mu)
	return out
}

// Write implements io.Writer.
func (b *StreamingActionBuffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		if b.err != nil {
			return 0, b.err
		}
		return 0, io.ErrClosedPipe
	}
	n, err := b.buf.Write(p)
	b.cond.Broadcast()
	return n, err
}

// Read implements io.Reader.
func (b *StreamingActionBuffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for b.buf.Len() == 0 && !b.closed {
		b.cond.Wait()
	}
	if b.buf.Len() > 0 {
		n, err := b.buf.Read(p)
		return n, err
	}
	if b.err != nil {
		return 0, b.err
	}
	return 0, io.EOF
}

// Close implements io.Closer.
func (b *StreamingActionBuffer) Close() error {
	return b.CloseWithError(nil)
}

// CloseWithError closes the buffer with an error.
func (b *StreamingActionBuffer) CloseWithError(err error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	b.err = err
	b.cond.Broadcast()
	return nil
}
