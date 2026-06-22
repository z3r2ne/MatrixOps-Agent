package semreg_runner

import (
	"strings"
	"sync"
	"time"

	agenttypes "matrixops-agent/types"
	"pkgs/db/models"
	"pkgs/semreg"
	"matrixops/types"
)

type TraceCollector struct {
	mu    sync.Mutex
	tools []traceToolRecord
}

type traceToolRecord struct {
	Tool        string
	Status      string
	Input       map[string]interface{}
	OutputChars int
	PartID      string
	StartedAt   int64
	EndedAt     int64
}

func NewTraceCollector() *TraceCollector {
	return &TraceCollector{}
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

func (h *TraceCollector) recordPart(part *agenttypes.Part) {
	if part == nil || part.Type != agenttypes.PartTypeTool || part.Tool == nil {
		return
	}
	name := strings.TrimSpace(part.Tool.Name)
	if name == "" {
		return
	}
	input := toolInputMap(part.Tool.State.Input)
	status := strings.TrimSpace(part.Tool.State.Status)
	outputChars := len(strings.TrimSpace(part.Tool.State.Output))
	now := time.Now().UnixMilli()

	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.tools {
		if h.tools[i].PartID == part.ID {
			h.tools[i].Status = status
			h.tools[i].OutputChars = outputChars
			if len(input) > 0 {
				h.tools[i].Input = input
			}
			if status == "completed" || status == "failed" || status == "error" {
				h.tools[i].EndedAt = now
			}
			return
		}
	}
	h.tools = append(h.tools, traceToolRecord{
		Tool:        name,
		Status:      status,
		Input:       input,
		OutputChars: outputChars,
		PartID:      part.ID,
		StartedAt:   now,
	})
}

func (h *TraceCollector) Snapshot() []semreg.TraceToolCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]semreg.TraceToolCall, len(h.tools))
	for i, call := range h.tools {
		out[i] = semreg.TraceToolCall{
			Tool:        call.Tool,
			Status:      call.Status,
			Input:       call.Input,
			OutputChars: call.OutputChars,
			TimeCreated: call.StartedAt,
		}
	}
	return out
}

func (h *TraceCollector) SnapshotDetailed() []map[string]interface{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]map[string]interface{}, len(h.tools))
	for i, call := range h.tools {
		out[i] = map[string]interface{}{
			"tool":        call.Tool,
			"status":      call.Status,
			"input":       call.Input,
			"outputChars": call.OutputChars,
			"partId":      call.PartID,
			"startedAt":   call.StartedAt,
			"endedAt":     call.EndedAt,
		}
	}
	return out
}

func (h *TraceCollector) BroadcastToTask(_ uint, msg types.WSOutgoingMessage) {
	if msg.Type != types.WSTypeMessageV2 {
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

func (h *TraceCollector) BroadcastTaskMessage(_ uint, _ *models.TaskMessage)              {}
func (h *TraceCollector) BroadcastNormalizedEntry(_ uint, _ *models.NormalizedEntry)       {}
func (h *TraceCollector) BroadcastTaskStatus(_ uint, _ models.TaskStatus, _ string, _ string) {}
func (h *TraceCollector) BroadcastIsWorking(_ uint)                                        {}
func (h *TraceCollector) BroadcastIsNotWorking(_ uint)                                     {}
func (h *TraceCollector) BroadcastStartLoading(_ uint)                                     {}
func (h *TraceCollector) BroadcastEndLoading(_ uint)                                       {}
func (h *TraceCollector) BroadcastError(_ uint, _ string)                                  {}
func (h *TraceCollector) BroadcastSessionTitle(_ uint, _ string)                           {}
func (h *TraceCollector) BroadcastRetry(_ uint)                                            {}
func (h *TraceCollector) BroadcastTaskQueue(_ uint, _ []models.TaskMessageQueueItem)        {}
func (h *TraceCollector) BroadcastTaskPlan(_ uint, _ any)                                  {}

func (h *TraceCollector) BroadcastWaitUserInput(_ uint, _ string, ack func(map[string]interface{}), _ map[string]interface{}) {
	if ack != nil {
		ack(map[string]interface{}{"decision": "allow"})
	}
}
