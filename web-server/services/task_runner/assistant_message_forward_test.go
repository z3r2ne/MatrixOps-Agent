package task_runner

import (
	"testing"

	agenttypes "matrixops-agent/types"
	"pkgs/db/models"
)

type taskMessageCaptureHub struct {
	stubWSHub
	taskMessages []*models.TaskMessage
}

func (h *taskMessageCaptureHub) BroadcastTaskMessage(_ uint, message *models.TaskMessage) {
	if message == nil {
		return
	}
	cloned := *message
	h.taskMessages = append(h.taskMessages, &cloned)
}

func TestShouldForwardAssistantMessage(t *testing.T) {
	if shouldForwardAssistantMessage(nil) {
		t.Fatal("nil info should not forward")
	}
	if shouldForwardAssistantMessage(&agenttypes.MessageInfo{Finish: "stop"}) {
		t.Fatal("in-progress finish should not forward")
	}
	if !shouldForwardAssistantMessage(&agenttypes.MessageInfo{Finish: "step-finish"}) {
		t.Fatal("step-finish should forward")
	}
	if !shouldForwardAssistantMessage(&agenttypes.MessageInfo{Time: agenttypes.MessageTime{Completed: 1}}) {
		t.Fatal("completed message should forward")
	}
}

func TestMaybeForwardAssistantMessage_EmitsTaskMessageOnce(t *testing.T) {
	hub := &taskMessageCaptureHub{}
	runtime := &TaskRuntime{
		taskID:                 1,
		emitter:                NewEmitter(hub, nil, 1),
		forwardedAssistantKeys: make(map[string]struct{}),
	}
	runtime.turnForwardToWeChat.Store(true)

	msg := &agenttypes.WithParts{
		Info: &agenttypes.MessageInfo{
			ID:     "msg-1",
			Role:   agenttypes.RoleAssistant,
			Finish: "step-finish",
		},
		Parts: []*agenttypes.Part{{
			Type: "text",
			Text: "你好，我是助手",
		}},
	}

	runtime.maybeForwardAssistantMessage(msg)
	runtime.maybeForwardAssistantMessage(msg)

	if len(hub.taskMessages) != 1 {
		t.Fatalf("task messages = %d, want 1", len(hub.taskMessages))
	}
	if hub.taskMessages[0].Role != "assistant" {
		t.Fatalf("role = %q, want assistant", hub.taskMessages[0].Role)
	}
	if hub.taskMessages[0].Content != "你好，我是助手" {
		t.Fatalf("content = %v", hub.taskMessages[0].Content)
	}
}

func TestMaybeForwardAssistantMessage_ForwardsAttachmentOnly(t *testing.T) {
	hub := &taskMessageCaptureHub{}
	runtime := &TaskRuntime{
		taskID:                           1,
		emitter:                          NewEmitter(hub, nil, 1),
		forwardedAssistantKeys:           make(map[string]struct{}),
		forwardedAssistantAttachmentKeys: make(map[string]struct{}),
	}
	runtime.turnForwardToWeChat.Store(true)

	runtime.maybeForwardAssistantMessage(&agenttypes.WithParts{
		Info: &agenttypes.MessageInfo{
			ID:     "msg-file",
			Role:   agenttypes.RoleAssistant,
			Finish: "step-finish",
		},
		Parts: []*agenttypes.Part{{
			ID:       "part-file",
			Type:     "file",
			URL:      "data:image/png;base64,abc",
			Filename: "demo.png",
			Mime:     "image/png",
		}},
	})

	if len(hub.taskMessages) != 1 {
		t.Fatalf("task messages = %d, want 1", len(hub.taskMessages))
	}
	if hub.taskMessages[0].Type != TaskMessageTypeAssistantAttachment {
		t.Fatalf("type = %q, want %q", hub.taskMessages[0].Type, TaskMessageTypeAssistantAttachment)
	}
}

func TestMaybeForwardAssistantMessage_SkipsWithoutTextOrAttachment(t *testing.T) {
	hub := &taskMessageCaptureHub{}
	runtime := &TaskRuntime{
		taskID:                           1,
		emitter:                          NewEmitter(hub, nil, 1),
		forwardedAssistantKeys:           make(map[string]struct{}),
		forwardedAssistantAttachmentKeys: make(map[string]struct{}),
	}

	runtime.maybeForwardAssistantMessage(&agenttypes.WithParts{
		Info: &agenttypes.MessageInfo{
			ID:     "msg-2",
			Role:   agenttypes.RoleAssistant,
			Finish: "step-finish",
		},
		Parts: []*agenttypes.Part{{
			Type: "tool",
		}},
	})

	if len(hub.taskMessages) != 0 {
		t.Fatalf("expected no task messages, got %d", len(hub.taskMessages))
	}
}

func TestMaybeForwardAssistantMessage_ForwardsExpandedTextAgain(t *testing.T) {
	hub := &taskMessageCaptureHub{}
	runtime := &TaskRuntime{
		taskID:                 1,
		emitter:                NewEmitter(hub, nil, 1),
		forwardedAssistantKeys: make(map[string]struct{}),
	}
	runtime.turnForwardToWeChat.Store(true)

	info := &agenttypes.MessageInfo{
		ID:     "msg-3",
		Role:   agenttypes.RoleAssistant,
		Finish: "step-finish",
	}

	runtime.maybeForwardAssistantMessage(&agenttypes.WithParts{
		Info:  info,
		Parts: []*agenttypes.Part{{Type: "text", Text: "第一段"}},
	})
	runtime.maybeForwardAssistantMessage(&agenttypes.WithParts{
		Info:  info,
		Parts: []*agenttypes.Part{{Type: "text", Text: "第一段\n第二段"}},
	})

	if len(hub.taskMessages) != 2 {
		t.Fatalf("task messages = %d, want 2", len(hub.taskMessages))
	}
}

func TestMaybeForwardAssistantMessage_SkipsFrontendTurn(t *testing.T) {
	hub := &taskMessageCaptureHub{}
	runtime := &TaskRuntime{
		taskID:                 1,
		emitter:                NewEmitter(hub, nil, 1),
		forwardedAssistantKeys: make(map[string]struct{}),
	}

	runtime.maybeForwardAssistantMessage(&agenttypes.WithParts{
		Info: &agenttypes.MessageInfo{
			ID:     "msg-frontend",
			Role:   agenttypes.RoleAssistant,
			Finish: "step-finish",
		},
		Parts: []*agenttypes.Part{{Type: "text", Text: "只给前端"}},
	})

	if len(hub.taskMessages) != 0 {
		t.Fatalf("frontend turn should not forward, got %d messages", len(hub.taskMessages))
	}
}
