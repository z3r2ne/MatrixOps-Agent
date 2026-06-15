package jsonextractor

import (
	"io"
	"sync"
)

// Pipe 创建一个内存管道
type Pipe struct {
	reader *PipeReader
	writer *PipeWriter
}

// PipeReader 管道读取端
type PipeReader struct {
	pipe *pipe
}

// PipeWriter 管道写入端
type PipeWriter struct {
	pipe *pipe
}

// pipe 内部实现
type pipe struct {
	mu       sync.Mutex
	cond     *sync.Cond
	buffer   []byte
	closed   bool
	writeErr error
}

// NewPipe 创建新的管道
func NewPipe() (*PipeReader, *PipeWriter) {
	p := &pipe{}
	p.cond = sync.NewCond(&p.mu)
	
	return &PipeReader{pipe: p}, &PipeWriter{pipe: p}
}

// Read 实现 io.Reader
func (r *PipeReader) Read(p []byte) (n int, err error) {
	r.pipe.mu.Lock()
	defer r.pipe.mu.Unlock()

	for len(r.pipe.buffer) == 0 && !r.pipe.closed {
		r.pipe.cond.Wait()
	}

	if len(r.pipe.buffer) > 0 {
		n = copy(p, r.pipe.buffer)
		r.pipe.buffer = r.pipe.buffer[n:]
		return n, nil
	}

	if r.pipe.closed {
		if r.pipe.writeErr != nil {
			return 0, r.pipe.writeErr
		}
		return 0, io.EOF
	}

	return 0, nil
}

// Close 关闭读取端
func (r *PipeReader) Close() error {
	r.pipe.mu.Lock()
	defer r.pipe.mu.Unlock()
	r.pipe.closed = true
	r.pipe.cond.Broadcast()
	return nil
}

// Write 实现 io.Writer
func (w *PipeWriter) Write(p []byte) (n int, err error) {
	w.pipe.mu.Lock()
	defer w.pipe.mu.Unlock()

	if w.pipe.closed {
		return 0, io.ErrClosedPipe
	}

	w.pipe.buffer = append(w.pipe.buffer, p...)
	n = len(p)
	w.pipe.cond.Broadcast()
	return n, nil
}

// Close 关闭写入端
func (w *PipeWriter) Close() error {
	w.pipe.mu.Lock()
	defer w.pipe.mu.Unlock()
	w.pipe.closed = true
	w.pipe.cond.Broadcast()
	return nil
}

// CloseWithError 关闭写入端并设置错误
func (w *PipeWriter) CloseWithError(err error) error {
	w.pipe.mu.Lock()
	defer w.pipe.mu.Unlock()
	w.pipe.closed = true
	w.pipe.writeErr = err
	w.pipe.cond.Broadcast()
	return nil
}
