package compatible

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode/utf8"

	agentprovider "matrixops-agent/provider"
	"matrixops.local/core_agent/streamtypes"
	"pkgs/jsonextractor"
)

const streamDiagWindowCap = 512

// streamByteWindow 保留最近若干字节，便于在流式解析失败时对照原始输出。
type streamByteWindow struct {
	buf [streamDiagWindowCap]byte
	n   int
	i   int
}

func (w *streamByteWindow) add(b byte) {
	w.buf[w.i] = b
	w.i = (w.i + 1) % streamDiagWindowCap
	if w.n < streamDiagWindowCap {
		w.n++
	}
}

func (w *streamByteWindow) String() string {
	if w.n == 0 {
		return ""
	}
	start := (w.i - w.n + streamDiagWindowCap) % streamDiagWindowCap
	out := make([]byte, 0, w.n)
	for j := 0; j < w.n; j++ {
		out = append(out, w.buf[(start+j)%streamDiagWindowCap])
	}
	return string(out)
}

func snippetAroundBytes(p []byte, center int, radius int) string {
	if len(p) == 0 {
		return ""
	}
	if center < 0 {
		center = 0
	}
	if center > len(p) {
		center = len(p)
	}
	start := center - radius
	if start < 0 {
		start = 0
	}
	end := center + radius
	if end > len(p) {
		end = len(p)
	}
	return string(p[start:end])
}

func truncateStringForLog(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	cut := s[:maxBytes]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return fmt.Sprintf("%s…(truncated, total %d bytes)", cut, len(s))
}

// parseActionStream 从 outer 解析一个或多个顶层 JSON 对象；每个对象为 {"@action":"<名称>","data":<任意 JSON>}。
func parseActionStream(outer *jsonextractor.PipeReader, actions chan<- *streamtypes.ActionOutput, setParseErr func(error)) {
	ParseActionStream(outer, actions, setParseErr)
}

func bridgeParsedActionsToToolCalls(
	parsed <-chan *streamtypes.ActionOutput,
	toolCalls chan<- *streamtypes.CallToolRequest,
	control streamtypes.CompatibleControlHandler,
	setParseErr func(error),
	actionEmitted *bool,
) {
	defer close(toolCalls)
	for action := range parsed {
		if action == nil {
			continue
		}
		if err := DispatchParsedAction(action, toolCalls, control); err != nil {
			setParseErr(err)
			return
		}
		if actionEmitted != nil {
			*actionEmitted = true
		}
	}
}

func dispatchParsedActionsBatch(
	parsed []*streamtypes.ActionOutput,
	toolCalls chan<- *streamtypes.CallToolRequest,
	control streamtypes.CompatibleControlHandler,
) error {
	for _, action := range parsed {
		if action == nil {
			continue
		}
		if err := DispatchParsedAction(action, toolCalls, control); err != nil {
			return err
		}
	}
	return nil
}

func isWhitespaceByte(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}


func StreamV2(input streamtypes.StreamInput, client streamtypes.ChatClient) (*streamtypes.StreamOutput, error) {
	return streamtypes.StreamWithRetries(input, func(attemptInput streamtypes.StreamInput) (*streamtypes.StreamOutput, error) {
		return streamV2Once(attemptInput, client)
	})
}

func streamV2Once(input streamtypes.StreamInput, client streamtypes.ChatClient) (*streamtypes.StreamOutput, error) {
	if input.Abort != nil {
		select {
		case <-input.Abort.Done():
			return nil, input.Abort.Err()
		default:
		}
	}

	userContent := any(input.Prompt)
	if len(input.UserContentParts) > 0 {
		parts := make([]agentprovider.CommonContentPart, 0, len(input.UserContentParts)+1)
		if strings.TrimSpace(input.Prompt) != "" {
			parts = append(parts, agentprovider.CommonContentPart{Type: "text", Text: input.Prompt})
		}
		parts = append(parts, input.UserContentParts...)
		if len(parts) > 0 {
			userContent = agentprovider.SimplifyTextOnlyContent(parts)
		}
	}

	messages := make([]*streamtypes.ModelMessage, 0, 2)
	if strings.TrimSpace(input.SystemPrompt) != "" {
		messages = append(messages, &streamtypes.ModelMessage{
			Role:    "system",
			Content: strings.TrimSpace(input.SystemPrompt),
		})
	}
	for _, historyMessage := range input.HistoryMessages {
		if historyMessage == nil {
			continue
		}
		messages = append(messages, historyMessage)
	}
	if len(input.UserContentParts) > 0 || strings.TrimSpace(input.Prompt) != "" || len(messages) == 0 {
		messages = append(messages, &streamtypes.ModelMessage{
			Role:    "user",
			Content: userContent,
		})
	}

	var extraOptions map[string]interface{}
	if strings.TrimSpace(input.Instruction) != "" {
		extraOptions = map[string]interface{}{
			"instructions": strings.TrimSpace(input.Instruction),
		}
	}

	req := streamtypes.ChatRequest{
		Context:         input.Context,
		Messages:        messages,
		Tools:           append([]streamtypes.ToolDefinition(nil), input.Tools...),
		ActionSchemas:   append([]streamtypes.ActionPromptSchema(nil), input.ActionSchemas...),
		Temperature:     input.Temperature,
		TopP:            input.TopP,
		MaxOutputTokens: input.MaxOutputTokens,
		ProviderOptions: input.ProviderOptions,
		Model:           input.Model,
		ExtraOptions:    extraOptions,
	}

	var usage *streamtypes.Usage
	var rawTextBuilder bytes.Buffer
	var rawResponsePayload string
	var lastStreamFinish string
	var (
		parseErr   error
		parseErrMu sync.Mutex
	)

	setParseErr := func(err error) {
		if err == nil || err == io.EOF {
			return
		}
		parseErrMu.Lock()
		defer parseErrMu.Unlock()
		if parseErr == nil {
			parseErr = err
		}
	}

	getParseErr := func() error {
		parseErrMu.Lock()
		defer parseErrMu.Unlock()
		return parseErr
	}

	if streamer, ok := client.(streamtypes.StreamChatClientWithOptions); ok {
		stream, err := streamer.StreamChatWithOptions(
			req,
			streamtypes.WithHTTPClient(input.HTTPClient),
			streamtypes.WithOnRawRequest(input.OnRawRequest),
			streamtypes.WithOnRawResponse(func(raw string) {
				rawResponsePayload = raw
				if input.OnRawResponse != nil {
					input.OnRawResponse(raw)
				}
			}),
		)
		if err != nil {
			return nil, fmt.Errorf("stream chat with options: %w", err)
		}

		reader, writer := jsonextractor.NewPipe()
		parsed := make(chan *streamtypes.ActionOutput, 64)
		toolCalls := make(chan *streamtypes.CallToolRequest, 64)
		parserDone := make(chan struct{})
		bridgeDone := make(chan struct{})
		streamDone := make(chan struct{})
		actionEmitted := false

		go func() {
			defer close(parserDone)
			parseActionStream(reader, parsed, setParseErr)
		}()

		go func() {
			defer close(bridgeDone)
			bridgeParsedActionsToToolCalls(parsed, toolCalls, input.CompatibleControlHandler, setParseErr, &actionEmitted)
		}()

		go func() {
			defer close(streamDone)
			defer writer.Close()
			for {
				var (
					event streamtypes.StreamEvent
					ok    bool
				)

				if input.Abort != nil {
					select {
					case <-input.Abort.Done():
						setParseErr(input.Abort.Err())
						return
					case event, ok = <-stream:
					}
				} else {
					event, ok = <-stream
				}

				if !ok {
					return
				}
				if event.Type == "error" {
					setParseErr(event.Error)
					return
				}

				switch event.Type {
				case "text-delta":
					if _, err := writer.Write([]byte(event.Text)); err != nil {
						setParseErr(fmt.Errorf("write to pipe: %w", err))
						return
					}
					rawTextBuilder.WriteString(event.Text)
				case "finish":
					if event.Usage != nil {
						usage = event.Usage
					}
					if fr := strings.TrimSpace(event.Finish); fr != "" {
						lastStreamFinish = fr
					}
				}
			}
		}()

		output := &streamtypes.StreamOutput{ToolCalls: toolCalls, RawTextReader: &rawTextBuilder}
		output.Wait = func() error {
			<-streamDone
			<-parserDone
			<-bridgeDone
			if err := getParseErr(); err != nil {
				if !errors.Is(err, io.ErrUnexpectedEOF) || !actionEmitted {
					return fmt.Errorf("parse JSON stream: %w", err)
				}
			}
			if err := streamtypes.RetryErrorForEmptyStreamOutput(lastStreamFinish, rawResponsePayload, rawTextBuilder.Len() > 0); err != nil {
				return fmt.Errorf("parse JSON stream: %w", err)
			}
			output.Usage = usage
			return nil
		}
		return output, nil
	}

	response, err := client.Chat(req)
	if err != nil {
		return nil, err
	}

	rawTextBuilder.WriteString(streamtypes.RenderContent(response.Message.Content))
	usage = response.Usage

	parsedActions, parseErr := ParseActionBytes(rawTextBuilder.Bytes())
	if parseErr != nil {
		toolCalls := make(chan *streamtypes.CallToolRequest)
		close(toolCalls)
		waitErr := fmt.Errorf("parse JSON stream: %w", parseErr)
		out := &streamtypes.StreamOutput{
			ToolCalls:     toolCalls,
			RawTextReader: &rawTextBuilder,
			Usage:         usage,
		}
		out.Wait = func() error { return waitErr }
		return out, nil
	}

	toolCalls := make(chan *streamtypes.CallToolRequest, len(parsedActions))
	if err := dispatchParsedActionsBatch(parsedActions, toolCalls, input.CompatibleControlHandler); err != nil {
		close(toolCalls)
		waitErr := fmt.Errorf("dispatch parsed actions: %w", err)
		out := &streamtypes.StreamOutput{
			ToolCalls:     toolCalls,
			RawTextReader: &rawTextBuilder,
			Usage:         usage,
		}
		out.Wait = func() error { return waitErr }
		return out, nil
	}
	close(toolCalls)

	return &streamtypes.StreamOutput{
		ToolCalls:     toolCalls,
		RawTextReader: &rawTextBuilder,
		Wait:          func() error { return nil },
		Usage:         usage,
	}, nil
}

