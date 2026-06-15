package generic

import (
	"encoding/json"
	"strings"
	"sync"

	coreagent "matrixops.local/core_agent"
)

type captureEmitter struct {
	inner   coreagent.Emitter
	mu      sync.Mutex
	parts   map[string]*coreagent.Part
	order   []string
	message map[string]*coreagent.Message
}

func newCaptureEmitter(inner coreagent.Emitter) *captureEmitter {
	if inner == nil {
		inner = coreagent.NoEmitter{}
	}
	return &captureEmitter{
		inner:   inner,
		parts:   map[string]*coreagent.Part{},
		order:   make([]string, 0, 16),
		message: map[string]*coreagent.Message{},
	}
}

func (e *captureEmitter) UpdateMessage(info *coreagent.Message) (*coreagent.Message, error) {
	if info != nil {
		e.mu.Lock()
		e.message[info.ID] = cloneCoreMessage(info)
		e.mu.Unlock()
	}
	return e.inner.UpdateMessage(info)
}

func (e *captureEmitter) UpdatePart(part *coreagent.Part) (*coreagent.Part, error) {
	if part != nil {
		e.mu.Lock()
		if _, ok := e.parts[part.ID]; !ok {
			e.order = append(e.order, part.ID)
		}
		e.parts[part.ID] = cloneCorePart(part)
		e.mu.Unlock()
	}
	return e.inner.UpdatePart(part)
}

func (e *captureEmitter) Emit(name string, payload interface{}) {
	e.inner.Emit(name, payload)
}

func (e *captureEmitter) partsForMessage(messageID string) []*coreagent.Part {
	e.mu.Lock()
	defer e.mu.Unlock()
	parts := make([]*coreagent.Part, 0, len(e.order))
	for _, id := range e.order {
		part := e.parts[id]
		if part == nil || part.MessageID != messageID {
			continue
		}
		parts = append(parts, cloneCorePart(part))
	}
	return parts
}

func cloneCoreMessage(message *coreagent.Message) *coreagent.Message {
	if message == nil {
		return nil
	}
	cloned := *message
	if message.Tools != nil {
		cloned.Tools = make(map[string]bool, len(message.Tools))
		for key, value := range message.Tools {
			cloned.Tools[key] = value
		}
	}
	if message.Tokens != nil {
		tokenCopy := *message.Tokens
		cloned.Tokens = &tokenCopy
	}
	if message.Error != nil {
		errorCopy := *message.Error
		if message.Error.ResponseHeaders != nil {
			errorCopy.ResponseHeaders = make(map[string]string, len(message.Error.ResponseHeaders))
			for key, value := range message.Error.ResponseHeaders {
				errorCopy.ResponseHeaders[key] = value
			}
		}
		cloned.Error = &errorCopy
	}
	if message.Path != nil {
		pathCopy := *message.Path
		cloned.Path = &pathCopy
	}
	cloned.Summary = cloneAnyValue(message.Summary)
	cloned.Memory = cloneAnyValue(message.Memory)
	return &cloned
}

func cloneCorePart(part *coreagent.Part) *coreagent.Part {
	if part == nil {
		return nil
	}
	cloned := *part
	if part.Metadata != nil {
		cloned.Metadata = cloneAnyMap(part.Metadata)
	}
	if len(part.Files) > 0 {
		cloned.Files = append([]string(nil), part.Files...)
	}
	cloned.Source = cloneAnyValue(part.Source)
	if part.Model != nil {
		modelCopy := *part.Model
		cloned.Model = &modelCopy
	}
	if part.Error != nil {
		errorCopy := *part.Error
		if part.Error.ResponseHeaders != nil {
			errorCopy.ResponseHeaders = make(map[string]string, len(part.Error.ResponseHeaders))
			for key, value := range part.Error.ResponseHeaders {
				errorCopy.ResponseHeaders[key] = value
			}
		}
		cloned.Error = &errorCopy
	}
	if part.Tokens != nil {
		tokenCopy := *part.Tokens
		cloned.Tokens = &tokenCopy
	}
	if part.Time != nil {
		timeCopy := *part.Time
		cloned.Time = &timeCopy
	}
	if part.Tool != nil {
		toolCopy := *part.Tool
		toolCopy.State = cloneCoreToolState(part.Tool.State)
		if part.Tool.Metadata != nil {
			toolCopy.Metadata = cloneAnyMap(part.Tool.Metadata)
		}
		cloned.Tool = &toolCopy
	}
	return &cloned
}

func cloneCoreToolState(state coreagent.ToolState) coreagent.ToolState {
	cloned := state
	cloned.Input = cloneAnyValue(state.Input)
	if state.Metadata != nil {
		cloned.Metadata = cloneAnyMap(state.Metadata)
	}
	if len(state.Attachments) > 0 {
		cloned.Attachments = make([]coreagent.Part, 0, len(state.Attachments))
		for _, attachment := range state.Attachments {
			if copied := cloneCorePart(&attachment); copied != nil {
				cloned.Attachments = append(cloned.Attachments, *copied)
			}
		}
	}
	if state.MemoryMetadata != nil {
		cloned.MemoryMetadata = cloneAnyMap(state.MemoryMetadata)
	}
	return cloned
}

func cloneAnyMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		out[key] = cloneAnyValue(value)
	}
	return out
}

func cloneAnyValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		return cloneAnyMap(typed)
	case []interface{}:
		out := make([]interface{}, len(typed))
		for index, item := range typed {
			out[index] = cloneAnyValue(item)
		}
		return out
	case []string:
		return append([]string(nil), typed...)
	case []coreagent.Part:
		out := make([]coreagent.Part, 0, len(typed))
		for _, item := range typed {
			if copied := cloneCorePart(&item); copied != nil {
				out = append(out, *copied)
			}
		}
		return out
	default:
		payload, err := json.Marshal(value)
		if err != nil {
			return value
		}
		var out interface{}
		if err := json.Unmarshal(payload, &out); err != nil {
			return value
		}
		return out
	}
}

func collectTextOutput(parts []*coreagent.Part) string {
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == nil || part.Type != coreagent.PartTypeText {
			continue
		}
		text := strings.TrimSpace(part.Text)
		if text == "" {
			continue
		}
		lines = append(lines, text)
	}
	return strings.Join(lines, "\n")
}
