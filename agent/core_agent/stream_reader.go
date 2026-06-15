package coreagent

import (
	"bytes"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

type StreamReaderCallback func(content string) error

var streamReaderSeq uint64

// StreamReader wraps an io.Reader and periodically reports the accumulated content.
type StreamReader struct {
	reader *StreamingActionBuffer
	id     uint64
}

// NewStreamReader creates a streaming reader with periodic callbacks.
func NewStreamReader(reader io.Reader, interval time.Duration, callback StreamReaderCallback) *StreamReader {
	streamReader := &StreamReader{
		reader: NewStreamingActionBuffer(),
		id:     atomic.AddUint64(&streamReaderSeq, 1),
	}

	go func() {
		var (
			contentMu    sync.Mutex
			callbackMu   sync.Mutex
			closeOnce    sync.Once
			content      bytes.Buffer
			lastEmitted  string
			callbackErr  error
			lifecycleEnd = make(chan struct{})
		)

		getCallbackErr := func() error {
			contentMu.Lock()
			defer contentMu.Unlock()
			return callbackErr
		}

		setCallbackErr := func(err error) {
			contentMu.Lock()
			defer contentMu.Unlock()
			if callbackErr == nil {
				callbackErr = err
			}
		}

		closePipe := func(err error) {
			closeOnce.Do(func() {
				close(lifecycleEnd)
				_ = streamReader.reader.CloseWithError(err)
			})
		}

		emit := func(force bool) error {
			if callback == nil {
				return nil
			}

			callbackMu.Lock()
			defer callbackMu.Unlock()

			contentMu.Lock()
			snapshot := content.String()
			previous := lastEmitted
			err := callbackErr
			contentMu.Unlock()

			if err != nil {
				return err
			}
			if !force && snapshot == previous {
				return nil
			}

			if err := callback(snapshot); err != nil {
				setCallbackErr(err)
				return err
			}

			contentMu.Lock()
			lastEmitted = snapshot
			contentMu.Unlock()
			return nil
		}

		if interval > 0 && callback != nil {
			go func() {
				ticker := time.NewTicker(interval)
				defer ticker.Stop()

				for {
					select {
					case <-lifecycleEnd:
						return
					case <-ticker.C:
						if err := emit(false); err != nil {
							closePipe(err)
							return
						}
					}
				}
			}()
		}

		buf := make([]byte, 32*1024)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				chunk := append([]byte(nil), buf[:n]...)

				contentMu.Lock()
				_, _ = content.Write(chunk)
				contentMu.Unlock()

				// NOTE: streamingActionBuffer 写入不会因“无人读取”而阻塞，
				// 因此 callback 可以独立工作，外部 Read 只是可选消费。
				if _, writeErr := streamReader.reader.Write(chunk); writeErr != nil {
					if callbackErr := getCallbackErr(); callbackErr != nil {
						closePipe(callbackErr)
					} else {
						closePipe(writeErr)
					}
					return
				}
			}

			if err == nil {
				continue
			}

			if callbackErr := getCallbackErr(); callbackErr != nil {
				closePipe(callbackErr)
				return
			}

			if err == io.EOF {
				closePipe(nil)
				if callback != nil {
					go func() {
						_ = emit(true)
					}()
				}
				return
			}

			closePipe(err)
			return
		}
	}()

	return streamReader
}

func (r *StreamReader) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}
