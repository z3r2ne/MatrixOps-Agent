package generic

import (
	"context"
	"errors"
	"strings"
	"testing"

	coreagent "matrixops.local/core_agent"
)

type scriptedStreamChatClient struct {
	stream chan coreagent.StreamEvent
}

func newScriptedStreamChatClient() *scriptedStreamChatClient {
	return &scriptedStreamChatClient{stream: make(chan coreagent.StreamEvent, 32)}
}

func (m *scriptedStreamChatClient) StreamChatWithOptions(req coreagent.ChatRequest, opts ...coreagent.StreamChatOption) (<-chan coreagent.StreamEvent, error) {
	options := coreagent.NewStreamChatOptions(opts...)
	if options.OnRequest != nil {
		if err := options.OnRequest(&req); err != nil {
			return nil, err
		}
	}
	return m.stream, nil
}

func (m *scriptedStreamChatClient) Chat(req coreagent.ChatRequest) (coreagent.ChatResponse, error) {
	return coreagent.ChatResponse{}, errors.New("not implemented")
}

func (m *scriptedStreamChatClient) sendText(text string) {
	m.stream <- coreagent.StreamEvent{Type: "text-delta", Text: text}
}

func (m *scriptedStreamChatClient) finish() {
	m.stream <- coreagent.StreamEvent{
		Type:   "finish",
		Finish: "stop",
		Usage:  &coreagent.Usage{InputTokens: 10, OutputTokens: 12},
	}
	close(m.stream)
}

// stubLLM 仅满足 New 校验；本测试只调用 buildPromptBuilder，不会发起对话。
type stubLLM struct{}

func (stubLLM) StreamChatWithOptions(coreagent.ChatRequest, ...coreagent.StreamChatOption) (<-chan coreagent.StreamEvent, error) {
	return nil, errors.New("stub: not used")
}

func (stubLLM) Chat(coreagent.ChatRequest) (coreagent.ChatResponse, error) {
	return coreagent.ChatResponse{}, errors.New("stub: not used")
}

func TestWorkerPromptBuilder_ComposesStaticSubAndMainSections(t *testing.T) {
	worker, err := NewFromLegacy(Config{
		LLMClient:        stubLLM{},
		WorkerPrompt:     "worker prompt",
		ModelPrompt:      "model prompt",
		OccupationPrompt: "occupation prompt",
		StaticPrompts: []StaticPromptSection{
			{Name: "project_prompt", Content: "project prompt"},
		},
		SubPromptBuilders: []PromptSection{
			{
				Name: "sub_prompt",
				Builder: func(state *coreagent.RunState) (string, error) {
					return "sub prompt", nil
				},
			},
		},
		MainPromptBuilder: func(state *coreagent.RunState) (string, error) {
			return "main prompt", nil
		},
	})
	if err != nil {
		t.Fatalf("NewFromLegacy: %v", err)
	}

	text, err := worker.buildPromptBuilder()(&coreagent.RunState{})
	if err != nil {
		t.Fatalf("buildPromptBuilder returned error: %v", err)
	}

	for _, expected := range []string{
		"<worker_prompt>",
		"worker prompt",
		"<model_prompt>",
		"<occupation_prompt>",
		"<project_prompt>",
		"<sub_prompt>",
		"main prompt",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected composed prompt to contain %q, got:\n%s", expected, text)
		}
	}
}

func TestNew_DefaultOptionsProducesRunnableWorker(t *testing.T) {
	w, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if w == nil || w.Name() != "" {
		t.Fatalf("unexpected worker: %#v", w)
	}
}

func TestWorkerRunTask_ReturnsAssistantTextAndUsesMemorySystem(t *testing.T) {
	client := newScriptedStreamChatClient()
	worker, err := NewFromLegacy(Config{
		Name:      "generic-chat",
		LLMClient: client,
		Model:     "gpt-test",
		MemorySystem: MemorySystem{
			Build: func(state *coreagent.RunState) (any, error) {
				return "memory payload", nil
			},
		},
		MainPromptBuilder: func(state *coreagent.RunState) (string, error) {
			if state.Memory != "memory payload" {
				t.Fatalf("expected state.Memory to be built before prompt, got %#v", state.Memory)
			}
			return "prompt", nil
		},
	})
	if err != nil {
		t.Fatalf("NewFromLegacy: %v", err)
	}

	done := make(chan struct {
		output string
		err    error
	}, 1)
	go func() {
		output, runErr := worker.RunTask(context.Background(), "hello")
		done <- struct {
			output string
			err    error
		}{output: output, err: runErr}
	}()

	client.sendText(`{"@action":"answer","data":"done"}`)
	client.finish()

	result := <-done
	if result.err != nil {
		t.Fatalf("RunTask returned error: %v", result.err)
	}
	if result.output != "done" {
		t.Fatalf("RunTask output = %q, want %q", result.output, "done")
	}
}
