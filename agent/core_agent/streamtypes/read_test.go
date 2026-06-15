package streamtypes

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"
)

func TestStreamOutputReadConsumesReasonContentAndToolCallsConcurrently(t *testing.T) {
	reasonReader, reasonWriter := io.Pipe()
	contentReader, contentWriter := io.Pipe()
	toolCalls := make(chan *CallToolRequest, 1)

	out := &StreamOutput{
		ToolCalls:     toolCalls,
		ReasonReader:  reasonReader,
		ContentReader: contentReader,
		Wait:          func() error { return nil },
	}

	reasonSeen := make(chan []byte, 2)
	contentSeen := make(chan []byte, 2)
	argsPayload := `{"path":"README.md"}`

	go func() {
		toolCalls <- &CallToolRequest{Index: 0, Name: "read", Arguments: bytes.NewBufferString(argsPayload)}
		time.Sleep(20 * time.Millisecond)
		_, _ = reasonWriter.Write([]byte("rea"))
		_, _ = contentWriter.Write([]byte("con"))
		time.Sleep(20 * time.Millisecond)
		_, _ = reasonWriter.Write([]byte("son"))
		_, _ = contentWriter.Write([]byte("tent"))
		_ = reasonWriter.Close()
		_ = contentWriter.Close()
		close(toolCalls)
	}()

	var started *CallToolRequest
	err := out.Read(StreamReadOptions{
		Interval: 10 * time.Millisecond,
		OnReason: func(b []byte) error {
			reasonSeen <- append([]byte(nil), b...)
			return nil
		},
		OnContent: func(b []byte) error {
			contentSeen <- append([]byte(nil), b...)
			return nil
		},
		OnToolCall: func(req *CallToolRequest) error {
			started = req
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if started == nil {
		t.Fatal("expected OnToolCall to be called")
	}
	reasonLast := <-reasonSeen
	for len(reasonSeen) > 0 {
		reasonLast = <-reasonSeen
	}
	if string(reasonLast) != "reason" {
		t.Fatalf("reason snapshot = %q", string(reasonLast))
	}
	contentLast := <-contentSeen
	for len(contentSeen) > 0 {
		contentLast = <-contentSeen
	}
	if string(contentLast) != "content" {
		t.Fatalf("content snapshot = %q", string(contentLast))
	}
	payload, readErr := io.ReadAll(started.Arguments)
	if readErr != nil {
		t.Fatalf("read completed tool args: %v", readErr)
	}
	if string(payload) != argsPayload {
		t.Fatalf("completed tool args = %q", string(payload))
	}
}

type blockingReader struct {
	onFirstRead func()
	once        sync.Once
	release     <-chan struct{}
}

func (r *blockingReader) Read(p []byte) (int, error) {
	r.once.Do(func() {
		if r.onFirstRead != nil {
			r.onFirstRead()
		}
	})
	<-r.release
	return 0, io.EOF
}

func TestReadToolCallStreamRunsCallsConcurrently(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})

	toolCalls := make(chan *CallToolRequest, 2)
	toolCalls <- &CallToolRequest{Index: 0, Name: "a", Arguments: bytes.NewBufferString(`{}`)}
	toolCalls <- &CallToolRequest{Index: 1, Name: "b", Arguments: bytes.NewBufferString(`{}`)}
	close(toolCalls)

	done := make(chan error, 1)
	go func() {
		done <- readToolCallStream(toolCalls, StreamReadOptions{
			OnToolCall: func(req *CallToolRequest) error {
				started <- req.Name
				<-release
				return nil
			},
		})
	}()

	seen := map[string]bool{}
	timeout := time.After(500 * time.Millisecond)
	for len(seen) < 2 {
		select {
		case name := <-started:
			seen[name] = true
		case <-timeout:
			t.Fatal("expected both tool calls to start before release")
		}
	}

	close(release)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("readToolCallStream: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for parallel tool executions to finish")
	}
}

func TestStreamOutputReadStaggersReasonContentAndToolCalls(t *testing.T) {
	reasonRelease := make(chan struct{})
	contentRelease := make(chan struct{})
	toolCallRelease := make(chan struct{})
	toolCalls := make(chan *CallToolRequest, 1)
	toolCalls <- &CallToolRequest{Index: 0, Name: "read", Arguments: bytes.NewBufferString(`{"path":"a"}`)}

	var (
		mu            sync.Mutex
		reasonStart   time.Time
		contentStart  time.Time
		toolCallStart time.Time
	)
	record := func(target *time.Time) func() {
		return func() {
			mu.Lock()
			defer mu.Unlock()
			*target = time.Now()
		}
	}

	out := &StreamOutput{
		ReasonReader:  &blockingReader{onFirstRead: record(&reasonStart), release: reasonRelease},
		ContentReader: &blockingReader{onFirstRead: record(&contentStart), release: contentRelease},
		ToolCalls:     toolCalls,
	}

	done := make(chan error, 1)
	interval := 20 * time.Millisecond
	go func() {
		done <- out.Read(StreamReadOptions{
			Interval: interval,
			OnToolCall: func(req *CallToolRequest) error {
				mu.Lock()
				toolCallStart = time.Now()
				mu.Unlock()
				<-toolCallRelease
				return nil
			},
		})
	}()

	time.Sleep(90 * time.Millisecond)
	close(reasonRelease)
	close(contentRelease)
	close(toolCallRelease)
	close(toolCalls)

	if err := <-done; err != nil {
		t.Fatalf("Read returned error: %v", err)
	}

	mu.Lock()
	gotReasonStart := reasonStart
	gotContentStart := contentStart
	gotToolCallStart := toolCallStart
	mu.Unlock()

	if gotReasonStart.IsZero() || gotContentStart.IsZero() || gotToolCallStart.IsZero() {
		t.Fatalf("unexpected zero start times: reason=%v content=%v toolCall=%v", gotReasonStart, gotContentStart, gotToolCallStart)
	}
	if !gotReasonStart.Before(gotContentStart) {
		t.Fatalf("expected reason to start before content: reason=%v content=%v", gotReasonStart, gotContentStart)
	}
	if !gotContentStart.Before(gotToolCallStart) {
		t.Fatalf("expected content to start before tool calls: content=%v toolCall=%v", gotContentStart, gotToolCallStart)
	}

	contentDelay := gotContentStart.Sub(gotReasonStart)
	toolCallDelay := gotToolCallStart.Sub(gotContentStart)
	minGap := time.Duration(float64(interval) * 1.1)
	if contentDelay < minGap {
		t.Fatalf("content started too early: delay=%v, want >= %v", contentDelay, minGap)
	}
	if toolCallDelay < minGap {
		t.Fatalf("tool calls started too early: delay=%v, want >= %v", toolCallDelay, minGap)
	}
}
