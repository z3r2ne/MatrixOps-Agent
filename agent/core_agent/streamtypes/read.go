package streamtypes

import (
	"bytes"
	"io"
	"sync"
	"time"
)

const streamReadStartDelayFactor = 1.5

type StreamReadOptions struct {
	Interval   time.Duration
	OnReason   func([]byte) error
	OnContent  func([]byte) error
	OnToolCall func(*CallToolRequest) error
}

func (o *StreamOutput) Read(options StreamReadOptions) error {
	if o == nil {
		return nil
	}

	var (
		wg          sync.WaitGroup
		waitErr     error
		reasonErr   error
		contentErr  error
		toolCallErr error
	)

	if o.Wait != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			waitErr = o.Wait()
		}()
	}

	if o.ReasonReader != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, reasonErr = readStreamReader(o.ReasonReader, options.Interval, options.OnReason)
		}()
	}

	if o.ContentReader != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sleepStreamReadStartDelay(options.Interval, 1)
			_, contentErr = readStreamReader(o.ContentReader, options.Interval, options.OnContent)
		}()
	}

	if o.ToolCalls != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sleepStreamReadStartDelay(options.Interval, 2)
			toolCallErr = readToolCallStream(o.ToolCalls, options)
		}()
	}

	wg.Wait()

	if toolCallErr != nil {
		return toolCallErr
	}
	if reasonErr != nil {
		return reasonErr
	}
	if contentErr != nil {
		return contentErr
	}
	if waitErr != nil {
		return waitErr
	}
	return nil
}

func sleepStreamReadStartDelay(interval time.Duration, phaseIndex int) {
	if interval <= 0 || phaseIndex <= 0 {
		return
	}
	delay := time.Duration(float64(interval) * streamReadStartDelayFactor * float64(phaseIndex))
	if delay <= 0 {
		return
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	<-timer.C
}

func readToolCallStream(toolCalls <-chan *CallToolRequest, options StreamReadOptions) error {
	if toolCalls == nil {
		return nil
	}
	if options.OnToolCall == nil {
		for range toolCalls {
		}
		return nil
	}

	requests := make([]*CallToolRequest, 0)
	for req := range toolCalls {
		if req != nil {
			requests = append(requests, req)
		}
	}
	if len(requests) == 0 {
		return nil
	}

	return runIndexedParallel(len(requests), MaxConcurrentToolCalls, func(index int) error {
		return options.OnToolCall(requests[index])
	})
}

func readStreamReader(reader io.Reader, interval time.Duration, callback func([]byte) error) ([]byte, error) {
	if reader == nil {
		return nil, nil
	}

	var (
		mu            sync.Mutex
		content       bytes.Buffer
		lastEmitted   []byte
		firstErr      error
		callbackAlive = true
		done          = make(chan struct{})
	)

	setErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
		callbackAlive = false
	}

	emit := func(force bool) {
		if callback == nil {
			return
		}
		mu.Lock()
		if !callbackAlive {
			mu.Unlock()
			return
		}
		snapshot := append([]byte(nil), content.Bytes()...)
		same := bytes.Equal(snapshot, lastEmitted)
		mu.Unlock()
		if !force && same {
			return
		}
		if err := callback(snapshot); err != nil {
			setErr(err)
			return
		}
		mu.Lock()
		lastEmitted = snapshot
		mu.Unlock()
	}

	if interval > 0 && callback != nil {
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					emit(false)
				}
			}
		}()
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			mu.Lock()
			_, _ = content.Write(buf[:n])
			mu.Unlock()
		}
		if err == nil {
			continue
		}
		close(done)
		if err == io.EOF {
			emit(true)
			mu.Lock()
			out := append([]byte(nil), content.Bytes()...)
			cbErr := firstErr
			mu.Unlock()
			return out, cbErr
		}
		setErr(err)
		mu.Lock()
		out := append([]byte(nil), content.Bytes()...)
		cbErr := firstErr
		mu.Unlock()
		return out, cbErr
	}
}
