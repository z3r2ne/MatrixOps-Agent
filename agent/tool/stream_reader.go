package tool

import (
	"bytes"
	"io"
	"sync"
	"time"

	"pkgs/jsonextractor"
)

// DefaultStreamReaderInterval matches core_agent.DefaultStreamReaderInterval.
const DefaultStreamReaderInterval = 100 * time.Millisecond

type StreamDeltaCallback func(delta string) error

// DeltaStreamReader wraps an io.Reader and periodically reports newly read bytes.
type DeltaStreamReader struct {
	reader *jsonextractor.PipeReader
}

// NewDeltaStreamReader creates a streaming reader that invokes callback with output
// deltas on the given interval, plus a final flush when the source reaches EOF.
func NewDeltaStreamReader(reader io.Reader, interval time.Duration, callback StreamDeltaCallback) *DeltaStreamReader {
	pipeReader, pipeWriter := jsonextractor.NewPipe()
	streamReader := &DeltaStreamReader{reader: pipeReader}

	go func() {
		var (
			contentMu     sync.Mutex
			callbackMu    sync.Mutex
			closeOnce     sync.Once
			content       bytes.Buffer
			lastEmittedLen int
			callbackErr   error
			lifecycleEnd  = make(chan struct{})
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
				if err != nil {
					_ = pipeWriter.CloseWithError(err)
					return
				}
				_ = pipeWriter.Close()
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
			previousLen := lastEmittedLen
			err := callbackErr
			contentMu.Unlock()

			if err != nil {
				return err
			}

			delta := ""
			if len(snapshot) > previousLen {
				delta = snapshot[previousLen:]
			}
			if delta == "" && !force {
				return nil
			}
			if delta == "" {
				return nil
			}

			if err := callback(delta); err != nil {
				setCallbackErr(err)
				return err
			}

			contentMu.Lock()
			lastEmittedLen = len(snapshot)
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

				if _, writeErr := pipeWriter.Write(chunk); writeErr != nil {
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
				if emitErr := emit(true); emitErr != nil {
					closePipe(emitErr)
					return
				}
				closePipe(nil)
				return
			}

			closePipe(err)
			return
		}
	}()

	return streamReader
}

func (r *DeltaStreamReader) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}
