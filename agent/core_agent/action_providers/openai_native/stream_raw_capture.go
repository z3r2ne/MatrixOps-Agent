package openai_native

import (
	"bytes"
	"io"
	"sync"
)
type rawResponseCaptureReadCloser struct {
	inner    io.ReadCloser
	callback func(string)

	mu     sync.Mutex
	buf    bytes.Buffer
	closed bool
}

func newRawResponseCaptureReadCloser(inner io.ReadCloser, callback func(string)) io.ReadCloser {
	if inner == nil {
		return nil
	}
	return &rawResponseCaptureReadCloser{
		inner:    inner,
		callback: callback,
	}
}

func (r *rawResponseCaptureReadCloser) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	if n > 0 {
		r.mu.Lock()
		_, _ = r.buf.Write(p[:n])
		r.mu.Unlock()
	}
	if err == io.EOF {
		r.emit()
	}
	return n, err
}

func (r *rawResponseCaptureReadCloser) Close() error {
	err := r.inner.Close()
	r.emit()
	return err
}

func (r *rawResponseCaptureReadCloser) emit() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	payload := r.buf.String()
	callback := r.callback
	r.mu.Unlock()

	if callback != nil {
		callback(payload)
	}
}
