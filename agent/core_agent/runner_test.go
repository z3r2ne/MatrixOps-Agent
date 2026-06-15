package coreagent

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"pkgs/db/models"
)

type testEmitter struct {
	messages []*Message
	parts    []*Part
	footers  []AssistantFooterStatusPayload
}

type noopTool struct {
	name string
}

type stubActionProvider struct {
	stream func(StreamInput) (*StreamOutput, error)
}

func (p *stubActionProvider) Stream(input StreamInput) (*StreamOutput, error) {
	if p == nil || p.stream == nil {
		return nil, errors.New("stub action provider missing stream func")
	}
	return p.stream(input)
}

func (t *noopTool) Name() string { return t.name }

func (t *noopTool) Description() string { return "noop tool" }

func (t *noopTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"properties":           map[string]interface{}{},
		"additionalProperties": true,
	}
}

func (t *noopTool) Execute(ctx ToolContext, input map[string]interface{}) (ToolResult, error) {
	return ToolResult{Name: t.name, Content: "ok"}, nil
}

func (e *testEmitter) UpdateMessage(info *Message) (*Message, error) {
	copied := *info
	e.messages = append(e.messages, &copied)
	return info, nil
}

func (e *testEmitter) UpdatePart(part *Part) (*Part, error) {
	copied := *part
	e.parts = append(e.parts, &copied)
	return part, nil
}

func (e *testEmitter) Emit(name string, payload interface{}) {
	if name != EventAssistantFooterStatus {
		return
	}
	event, ok := payload.(AssistantFooterStatusPayload)
	if !ok {
		return
	}
	e.footers = append(e.footers, event)
}

type noActionRawResponseClient struct {
	stream chan StreamEvent
}

type multiStepStreamChatClient struct {
	mu       sync.Mutex
	requests int
}

func newNoActionRawResponseClient() *noActionRawResponseClient {
	return &noActionRawResponseClient{stream: make(chan StreamEvent, 1)}
}

func (c *multiStepStreamChatClient) StreamChatWithOptions(req ChatRequest, opts ...StreamChatOption) (<-chan StreamEvent, error) {
	c.mu.Lock()
	c.requests++
	requestNum := c.requests
	c.mu.Unlock()

	stream := make(chan StreamEvent, 2)
	go func() {
		defer close(stream)
		if requestNum == 1 {
			stream <- StreamEvent{Type: "text-delta", Text: `{"@action":"call_tool","data":{"name":"read","params":{"path":"README.md"}}}`}
			stream <- StreamEvent{Type: "finish", Finish: "stop"}
			return
		}
		stream <- StreamEvent{Type: "text-delta", Text: `{"@action":"answer","data":"done"}`}
		stream <- StreamEvent{Type: "finish", Finish: "stop"}
	}()
	return stream, nil
}

func (c *multiStepStreamChatClient) Chat(req ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, errors.New("not implemented")
}

func (c *noActionRawResponseClient) StreamChatWithOptions(req ChatRequest, opts ...StreamChatOption) (<-chan StreamEvent, error) {
	options := NewStreamChatOptions(opts...)
	if options.OnRawResponse != nil {
		options.OnRawResponse(`{"id":"resp_1","output":[]}`)
	}
	stream := make(chan StreamEvent, 1)
	stream <- StreamEvent{Type: "finish", Finish: "stop"}
	close(stream)
	return stream, nil
}

func (c *noActionRawResponseClient) Chat(req ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, errors.New("not implemented")
}

func TestCurrentIterationUserInput_NativeOpenAIToolCallsOnlyUsesInitialTurn(t *testing.T) {
	state := &RunState{
		UserInput:             "使用explore探索一下当前项目",
		NativeOpenAIToolCalls: true,
	}

	if got := currentIterationUserInput(state, 1); got != "使用explore探索一下当前项目" {
		t.Fatalf("step 1 user input = %q, want original input", got)
	}
	if got := currentIterationUserInput(state, 2); got != "" {
		t.Fatalf("step 2 user input = %q, want empty continuation input", got)
	}
}

func TestCurrentIterationUserInput_KeepsPromptForNonNativeOrFinalPass(t *testing.T) {
	state := &RunState{
		UserInput:                  "继续",
		NativeOpenAIToolCalls:      true,
		MaxStepsExhaustedFinalPass: true,
	}
	if got := currentIterationUserInput(state, 2); got != "继续" {
		t.Fatalf("final pass user input = %q, want original input", got)
	}

	state = &RunState{
		UserInput:             "继续",
		NativeOpenAIToolCalls: false,
	}
	if got := currentIterationUserInput(state, 2); got != "继续" {
		t.Fatalf("non-native user input = %q, want original input", got)
	}
}

func TestRunner_RunStopsOnAnswer(t *testing.T) {
	emitter := &testEmitter{}
	client := newScriptedStreamChatClient()
	runner, err := NewRunner(RunnerConfig{
		Emitter:       emitter,
		LLMClient:     client,
		PromptBuilder: func(state *RunState) (string, error) { return "prompt", nil },
		MaxSteps:      3,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	done := make(chan error, 1)
	go func() {
		done <- runner.Run(state)
	}()

	client.sendText(`{"@action":"answer","data":"done"}`)
	client.finish()

	if err := <-done; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if state.Assistant.Finish != "answer" {
		t.Fatalf("expected assistant finish=answer, got %q", state.Assistant.Finish)
	}
	foundText := false
	for _, part := range emitter.parts {
		if part.Type == PartTypeText && part.Text == "done" {
			foundText = true
			break
		}
	}
	if !foundText {
		t.Fatal("expected emitted text part for answer")
	}
}

func TestRunner_RunCreatesNewAssistantMessagePerIteration(t *testing.T) {
	emitter := &testEmitter{}
	client := &multiStepStreamChatClient{}
	runner, err := NewRunner(RunnerConfig{
		Emitter:       emitter,
		LLMClient:     client,
		PromptBuilder: func(state *RunState) (string, error) { return "prompt", nil },
		MaxSteps:      3,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	runner.tools.Register(&noopTool{name: "read"})

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	if err := runner.Run(state); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(emitter.messages) < 3 {
		t.Fatalf("expected multiple assistant message updates, got %d", len(emitter.messages))
	}

	seen := make(map[string]*Message)
	order := make([]string, 0, len(emitter.messages))
	for _, message := range emitter.messages {
		if message == nil || message.Role != RoleAssistant {
			continue
		}
		if _, ok := seen[message.ID]; ok {
			continue
		}
		cloned := *message
		seen[message.ID] = &cloned
		order = append(order, message.ID)
	}
	if len(order) < 2 {
		t.Fatalf("expected at least two assistant messages, got %v", order)
	}
	first := seen[order[0]]
	second := seen[order[1]]
	if first == nil || second == nil {
		t.Fatalf("missing captured messages: %v", order)
	}
	if first.ID == second.ID {
		t.Fatalf("expected different assistant message ids, got %q", first.ID)
	}
	if second.ParentID != "" {
		t.Fatalf("second parent_id = %q, want empty", second.ParentID)
	}
	if state.Assistant == nil || state.Assistant.ID != second.ID {
		t.Fatalf("final assistant id = %v, want %q", state.Assistant, second.ID)
	}
}

func TestRunner_HandleMessage_objectData_setsText(t *testing.T) {
	emitter := &testEmitter{}
	client := newScriptedStreamChatClient()
	runner, err := NewRunner(RunnerConfig{
		Emitter:       emitter,
		LLMClient:     client,
		PromptBuilder: func(state *RunState) (string, error) { return "prompt", nil },
		MaxSteps:      3,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}
	payload := `{"message":"我先把几个关键实现文件串起来看。","next_step":"读取数据库路径"}`
	for i := 0; i < 32; i++ {
		parts, err := runner.HandleAction(state, &ActionOutput{
			Action: "message",
			Data:   bytes.NewReader([]byte(payload)),
		})
		if err != nil {
			t.Fatalf("iteration %d: HandleAction: %v", i, err)
		}
		if len(parts) != 1 || parts[0] == nil {
			t.Fatalf("iteration %d: expected one part", i)
		}
		if parts[0].Type != PartTypeText {
			t.Fatalf("iteration %d: expected text part, got %q", i, parts[0].Type)
		}
		if parts[0].Text == "" {
			t.Fatalf("iteration %d: empty message text (async race regression)", i)
		}
	}
}

func TestRunner_HandleAnswer_ObjectDataWithJSONContentTypeSetsText(t *testing.T) {
	emitter := &testEmitter{}
	client := newScriptedStreamChatClient()
	runner, err := NewRunner(RunnerConfig{
		Emitter:          emitter,
		LLMClient:        client,
		PromptBuilder:    func(state *RunState) (string, error) { return "prompt", nil },
		MaxSteps:         3,
		AnswerActionType: ActionDataTypeJSONContent,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	parts, err := runner.HandleAction(state, &ActionOutput{
		Action: "answer",
		Data:   bytes.NewReader([]byte(`{"content":"done"}`)),
	})
	if err != nil {
		t.Fatalf("HandleAction: %v", err)
	}
	if len(parts) != 1 || parts[0] == nil {
		t.Fatal("expected one part")
	}
	if parts[0].Type != PartTypeText {
		t.Fatalf("expected text part, got %q", parts[0].Type)
	}
	if parts[0].Text != "done" {
		t.Fatalf("expected answer text %q, got %q", "done", parts[0].Text)
	}
}

func TestRunner_HandleDirectRegistryTool_InvalidJSONDoesNotReturnFatalError(t *testing.T) {
	emitter := &testEmitter{}
	client := newScriptedStreamChatClient()
	runner, err := NewRunner(RunnerConfig{
		Emitter:       emitter,
		LLMClient:     client,
		PromptBuilder: func(state *RunState) (string, error) { return "prompt", nil },
		MaxSteps:      3,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	runner.tools.Register(&noopTool{name: "tree"})

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	parts, err := runner.executeToolCallRequest(state, &CallToolRequest{
		Name:      "tree",
		Arguments: bytes.NewReader([]byte("/tmp")),
	})
	if err != nil {
		t.Fatalf("executeToolCallRequest returned fatal error: %v", err)
	}
	if len(parts) != 1 || parts[0] == nil {
		t.Fatalf("expected one tool part, got %#v", parts)
	}
	part := parts[0]
	if part.Type != PartTypeTool {
		t.Fatalf("expected tool part, got %q", part.Type)
	}
	if part.Tool == nil {
		t.Fatal("expected tool payload to be present")
	}
	if part.Tool.State.Status != "error" {
		t.Fatalf("expected tool status error, got %q", part.Tool.State.Status)
	}
	if !strings.Contains(part.Tool.State.Error, `tool "tree" arguments`) {
		t.Fatalf("expected tool state error to mention arguments parse failure, got %q", part.Tool.State.Error)
	}
	if !strings.Contains(part.Tool.State.SystemMessage, `invalid character '/'`) {
		t.Fatalf("expected tool system message to contain original json parse error, got system=%q output=%q", part.Tool.State.SystemMessage, part.Tool.State.Output)
	}
}

func TestRunner_HandleMessage_PlainTextDataSetsText(t *testing.T) {
	emitter := &testEmitter{}
	client := newScriptedStreamChatClient()
	runner, err := NewRunner(RunnerConfig{
		Emitter:       emitter,
		LLMClient:     client,
		PromptBuilder: func(state *RunState) (string, error) { return "prompt", nil },
		MaxSteps:      3,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	parts, err := runner.HandleAction(state, &ActionOutput{
		Action: "message",
		Data:   bytes.NewReader([]byte("plain message")),
	})
	if err != nil {
		t.Fatalf("HandleAction: %v", err)
	}
	if len(parts) != 1 || parts[0] == nil {
		t.Fatal("expected one part")
	}
	if parts[0].Text != "plain message" {
		t.Fatalf("expected plain message text, got %q", parts[0].Text)
	}
}

func TestRunner_RunMissingActionLogsRawResponseAsError(t *testing.T) {
	emitter := &testEmitter{}
	client := newNoActionRawResponseClient()
	var llmInfo *LLMCallInfo
	runner, err := NewRunner(RunnerConfig{
		Emitter:       emitter,
		LLMClient:     client,
		PromptBuilder: func(state *RunState) (string, error) { return "prompt", nil },
		MaxSteps:      1,
		Hooks: RunnerHooks{
			AfterLLMCall: func(state *RunState, info *LLMCallInfo) error {
				llmInfo = info
				return nil
			},
		},
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	err = runner.Run(state)
	if err == nil {
		t.Fatal("expected missing action error")
	}
	if !strings.Contains(err.Error(), "stream ended without model output") {
		t.Fatalf("expected error about empty stream output, got %v", err)
	}
	if llmInfo == nil {
		t.Fatal("expected AfterLLMCall info")
	}
	if llmInfo.Error == nil {
		t.Fatal("expected AfterLLMCall error")
	}
	if llmInfo.RawResponse != `{"id":"resp_1","output":[]}` {
		t.Fatalf("unexpected raw response: %q", llmInfo.RawResponse)
	}
}

func TestRunner_ExecuteOnceStopsAfterSingleStep(t *testing.T) {
	emitter := &testEmitter{}
	client := newScriptedStreamChatClient()
	runner, err := NewRunner(RunnerConfig{
		Emitter:       emitter,
		LLMClient:     client,
		PromptBuilder: func(state *RunState) (string, error) { return "prompt", nil },
		MaxSteps:      5,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "hello",
	}

	done := make(chan error, 1)
	go func() {
		done <- runner.ExecuteOnce(state)
	}()

	client.sendText(`{"@action":"message","data":{"message":"working","next_step":"continue"}}`)
	client.finish()

	if err := <-done; err != nil {
		t.Fatalf("ExecuteOnce returned error: %v", err)
	}
	if state.Step != 1 {
		t.Fatalf("expected only one step, got %d", state.Step)
	}
	if state.Assistant.Finish != "step-finish" {
		t.Fatalf("expected assistant finish=step-finish, got %q", state.Assistant.Finish)
	}
}

func TestRunner_ExecuteOnce_NativeOpenAICommentaryAndToolPartsBothPresent(t *testing.T) {
	emitter := &testEmitter{}
	rt := &nativeRouteRecordingRoundTripper{
		payload: strings.Join([]string{
			`event: response.output_text.delta`,
			`data: {"type":"response.output_text.delta","delta":"我先快速梳理了仓库结构。","output_index":0,"item_id":"msg_1"}`,
			``,
			`event: response.output_text.done`,
			`data: {"type":"response.output_text.done","text":"我先快速梳理了仓库结构。","output_index":0,"item_id":"msg_1"}`,
			``,
			`event: response.output_item.done`,
			`data: {"type":"response.output_item.done","item":{"id":"msg_1","type":"message","status":"completed","content":[{"type":"output_text","text":"我先快速梳理了仓库结构。"}],"phase":"commentary","role":"assistant"},"output_index":0}`,
			``,
			`event: response.output_item.added`,
			`data: {"type":"response.output_item.added","item":{"id":"fc_1","type":"function_call","status":"in_progress","arguments":"","call_id":"call_1","name":"read"},"output_index":1}`,
			``,
			`event: response.function_call_arguments.delta`,
			`data: {"type":"response.function_call_arguments.delta","delta":"{\"path\":\"README.md\"}","item_id":"fc_1","output_index":1}`,
			``,
			`event: response.function_call_arguments.done`,
			`data: {"type":"response.function_call_arguments.done","arguments":"{\"path\":\"README.md\"}","item_id":"fc_1","output_index":1}`,
			``,
			`event: response.completed`,
			`data: {"type":"response.completed","response":{"stop_reason":"stop","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2,"input_tokens_details":{"cached_tokens":0},"output_tokens_details":{"reasoning_tokens":0}}}}`,
			``,
		}, "\n"),
	}
	httpClient := &http.Client{Transport: rt}

	runner, err := NewRunner(RunnerConfig{
		Emitter:               emitter,
		LLMClient:             newNoActionRawResponseClient(),
		PromptBuilder:         func(state *RunState) (string, error) { return "prompt", nil },
		NativeOpenAIToolCalls: true,
		Model:                 "gpt-test",
		ProviderOptions: &models.LLMConfig{
			Name:       "openai",
			Type:       "openai",
			BaseURL:    "https://example.com/v1",
			APIKey:     "test-key",
			APIType:    models.LLMAPITypeResponse,
			MaxRetries: 1,
		},
		MaxSteps: 1,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	runner.RegisterTool(&noopTool{name: "read"})

	state := &RunState{
		Context:    context.Background(),
		SessionID:  "session-1",
		Assistant:  &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput:  "hello",
		HTTPClient: httpClient,
		Tools: []ToolDefinition{{
			Name:        "read",
			Description: "read tool",
			Schema:      map[string]interface{}{"type": "object"},
		}},
	}

	if err := runner.ExecuteOnce(state); err != nil {
		t.Fatalf("ExecuteOnce returned error: %v", err)
	}

	textIndex := -1
	toolIndex := -1
	for index, part := range emitter.parts {
		if part == nil {
			continue
		}
		if textIndex < 0 && part.Type == PartTypeText && strings.Contains(part.Text, "梳理了仓库结构") {
			textIndex = index
		}
		if toolIndex < 0 && part.Type == PartTypeTool && part.Tool != nil && part.Tool.Name == "read" {
			toolIndex = index
		}
	}
	if textIndex < 0 {
		t.Fatalf("expected commentary text part, got parts: %#v", emitter.parts)
	}
	if toolIndex < 0 {
		t.Fatalf("expected read tool part, got parts: %#v", emitter.parts)
	}
}

func TestRunner_ExecuteOnce_DoesNotFinishAnswerWhenContentPrecedesToolUse(t *testing.T) {
	emitter := &testEmitter{}
	provider := &stubActionProvider{
		stream: func(input StreamInput) (*StreamOutput, error) {
			toolCalls := make(chan *CallToolRequest, 1)
			toolCalls <- &CallToolRequest{
				Index:     0,
				Name:      "read",
				Arguments: bytes.NewReader([]byte(`{"path":"README.md"}`)),
				RawJSON:   `{"@action":"call_tool","data":{"name":"read","params":{"path":"README.md"}}}`,
			}
			close(toolCalls)
			return &StreamOutput{
				ToolCalls: toolCalls,
				RawTextReader: strings.NewReader(""),
				ContentReader: strings.NewReader("我先了解一下项目结构。"),
				Wait:          func() error { return nil },
			}, nil
		},
	}

	runner, err := NewRunner(RunnerConfig{
		Emitter:        emitter,
		PromptBuilder:  func(state *RunState) (string, error) { return "prompt", nil },
		MaxSteps:       5,
		ActionProvider: provider,
		Tools: func() *ToolRegistry {
			reg := NewToolRegistry()
			reg.Register(&noopTool{name: "read"})
			return reg
		}(),
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "介绍下这个项目",
	}

	if err := runner.ExecuteOnce(state); err != nil {
		t.Fatalf("ExecuteOnce returned error: %v", err)
	}
	if state.Assistant.Finish != "step-finish" {
		t.Fatalf("expected assistant finish=step-finish, got %q", state.Assistant.Finish)
	}

	var finishPart *Part
	var textPart *Part
	for _, part := range emitter.parts {
		if part == nil {
			continue
		}
		switch part.Type {
		case PartTypeFinishStep:
			finishPart = part
		case PartTypeText:
			textPart = part
		}
	}
	if textPart == nil || textPart.Text != "我先了解一下项目结构。" {
		if textPart == nil {
			t.Fatal("expected commentary text part")
		}
		t.Fatalf("unexpected text part: %q", textPart.Text)
	}
	if finishPart == nil {
		t.Fatal("expected finish-step part")
	}
	if got, _ := finishPart.Metadata["finishReason"].(string); got != "step-complete" {
		t.Fatalf("finishReason = %q, want %q", got, "step-complete")
	}
	if got, _ := finishPart.Metadata["hasNativeTextAnswer"].(bool); got {
		t.Fatal("hasNativeTextAnswer = true, want false")
	}
	if got, _ := finishPart.Metadata["nativeTextFinishesTurn"].(bool); got {
		t.Fatal("nativeTextFinishesTurn = true, want false")
	}
}

func TestRunner_ExecuteOnce_FinishesAnswerWhenProviderMarksNativeTextCompletion(t *testing.T) {
	emitter := &testEmitter{}
	provider := &stubActionProvider{
		stream: func(input StreamInput) (*StreamOutput, error) {
			toolCalls := make(chan *CallToolRequest)
			close(toolCalls)
			return &StreamOutput{
				ToolCalls: toolCalls,
				RawTextReader:                   strings.NewReader(""),
				ContentReader:                   strings.NewReader("项目是一个 AI 驱动的任务平台。"),
				NativeAssistantTextFinishesTurn: true,
				Wait:                            func() error { return nil },
			}, nil
		},
	}

	runner, err := NewRunner(RunnerConfig{
		Emitter:        emitter,
		PromptBuilder:  func(state *RunState) (string, error) { return "prompt", nil },
		MaxSteps:       5,
		ActionProvider: provider,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
		UserInput: "介绍下这个项目",
	}

	if err := runner.ExecuteOnce(state); err != nil {
		t.Fatalf("ExecuteOnce returned error: %v", err)
	}
	if state.Assistant.Finish != "answer" {
		t.Fatalf("expected assistant finish=answer, got %q", state.Assistant.Finish)
	}

	var finishPart *Part
	for _, part := range emitter.parts {
		if part != nil && part.Type == PartTypeFinishStep {
			finishPart = part
		}
	}
	if finishPart == nil {
		t.Fatal("expected finish-step part")
	}
	if got, _ := finishPart.Metadata["finishReason"].(string); got != "native-text-answer" {
		t.Fatalf("finishReason = %q, want %q", got, "native-text-answer")
	}
	if got, _ := finishPart.Metadata["hasNativeTextAnswer"].(bool); !got {
		t.Fatal("hasNativeTextAnswer = false, want true")
	}
}

func TestActionContextUpdatePart_EmitsLiveToolFooterFromInputPreview(t *testing.T) {
	emitter := &testEmitter{}
	runner, err := NewRunner(RunnerConfig{
		Emitter:       emitter,
		LLMClient:     newNoActionRawResponseClient(),
		PromptBuilder: func(state *RunState) (string, error) { return "prompt", nil },
		MaxSteps:      1,
	})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	state := &RunState{
		Context:   context.Background(),
		SessionID: "session-1",
		Assistant: &Message{ID: "msg-1", SessionID: "session-1", Role: RoleAssistant, Time: MessageTime{Created: 1}},
	}
	ctx := &ActionContext{Runner: runner, State: state}
	part := ctx.NewToolPart("write")
	part.Tool.State.Status = "input-streaming"
	part.Tool.State.Metadata = map[string]interface{}{
		"inputPreview":   "{\"path\":\"a.md\",\"content\":\"hello\"}",
		"inputStreaming": true,
	}

	if err := ctx.UpdatePart(part); err != nil {
		t.Fatalf("UpdatePart: %v", err)
	}
	if len(emitter.footers) == 0 {
		t.Fatal("expected footer status event")
	}
	got := emitter.footers[len(emitter.footers)-1]
	if !strings.Contains(got.Text, "write") {
		t.Fatalf("footer text = %q, want tool name", got.Text)
	}
	if !strings.Contains(got.Text, "\"path\":\"a.md\"") {
		t.Fatalf("footer text = %q, want input preview", got.Text)
	}
	if got.Text == "正在处理模型输出…" {
		t.Fatalf("footer text = %q, should not stay at generic model-processing status", got.Text)
	}
	if !got.Loading {
		t.Fatalf("footer loading = false, want true for input-streaming status")
	}
}
