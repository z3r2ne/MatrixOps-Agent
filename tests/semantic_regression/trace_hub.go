package semantic_regression

import (
	"strings"
	"sync"

	agenttypes "matrixops-agent/types"
	"pkgs/db/models"
	"pkgs/semreg"
	wstypes "matrixops/types"
)

type traceHub struct {
	mu    sync.Mutex
	tools []traceToolRecord
}

type traceToolRecord struct {
	Tool        string
	Status      string
	Input       map[string]interface{}
	OutputChars int
	PartID      string
}

func toolInputMap(input interface{}) map[string]interface{} {
	switch v := input.(type) {
	case map[string]interface{}:
		return v
	case nil:
		return map[string]interface{}{}
	default:
		return map[string]interface{}{"raw": v}
	}
}

func (h *traceHub) recordPart(part *agenttypes.Part) {
	if part == nil || part.Type != agenttypes.PartTypeTool || part.Tool == nil {
		return
	}
	name := strings.TrimSpace(part.Tool.Name)
	if name == "" {
		return
	}
	input := toolInputMap(part.Tool.State.Input)
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.tools {
		if h.tools[i].PartID == part.ID {
			h.tools[i].Status = strings.TrimSpace(part.Tool.State.Status)
			h.tools[i].OutputChars = len(strings.TrimSpace(part.Tool.State.Output))
			if len(input) > 0 {
				h.tools[i].Input = input
			}
			return
		}
	}
	h.tools = append(h.tools, traceToolRecord{
		Tool:        name,
		Status:      strings.TrimSpace(part.Tool.State.Status),
		Input:       input,
		OutputChars: len(strings.TrimSpace(part.Tool.State.Output)),
		PartID:      part.ID,
	})
}

func (h *traceHub) BroadcastToTask(_ uint, msg wstypes.WSOutgoingMessage) {
	if msg.Type != wstypes.WSTypeMessageV2 {
		return
	}
	switch value := msg.Data.(type) {
	case *agenttypes.WithParts:
		for _, part := range value.Parts {
			h.recordPart(part)
		}
	case agenttypes.WithParts:
		for _, part := range value.Parts {
			h.recordPart(part)
		}
	}
}

func (h *traceHub) BroadcastTaskMessage(_ uint, _ *models.TaskMessage)              {}
func (h *traceHub) BroadcastNormalizedEntry(_ uint, _ *models.NormalizedEntry)       {}
func (h *traceHub) BroadcastTaskStatus(_ uint, _ models.TaskStatus, _ string, _ string) {}
func (h *traceHub) BroadcastIsWorking(_ uint)                                        {}
func (h *traceHub) BroadcastIsNotWorking(_ uint)                                     {}
func (h *traceHub) BroadcastStartLoading(_ uint)                                     {}
func (h *traceHub) BroadcastEndLoading(_ uint)                                       {}
func (h *traceHub) BroadcastError(_ uint, _ string)                                  {}
func (h *traceHub) BroadcastSessionTitle(_ uint, _ string)                           {}
func (h *traceHub) BroadcastRetry(_ uint)                                            {}
func (h *traceHub) BroadcastTaskQueue(_ uint, _ []models.TaskMessageQueueItem)        {}
func (h *traceHub) BroadcastTaskPlan(_ uint, _ any)                                  {}
func (h *traceHub) BroadcastWaitUserInput(_ uint, _ string, ack func(map[string]interface{}), _ map[string]interface{}) {
	if ack != nil {
		ack(map[string]interface{}{"decision": "allow"})
	}
}

func (h *traceHub) snapshot() []semreg.TraceToolCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]semreg.TraceToolCall, len(h.tools))
	for i, call := range h.tools {
		out[i] = semreg.TraceToolCall{
			Tool:        call.Tool,
			Status:      call.Status,
			Input:       call.Input,
			OutputChars: call.OutputChars,
		}
	}
	return out
}
