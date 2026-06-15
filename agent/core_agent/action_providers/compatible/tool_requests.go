package compatible

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"matrixops.local/core_agent/streamtypes"
)

var controlActions = map[string]struct{}{
	"message": {},
	"answer":  {},
}

// IsControlAction reports whether the action name is handled by CompatibleControlHandler.
func IsControlAction(name string) bool {
	_, ok := controlActions[strings.TrimSpace(name)]
	return ok
}

// ExpandCallToolRequests converts a parsed envelope into one or more CallToolRequest values.
func ExpandCallToolRequests(action *streamtypes.ActionOutput) ([]*streamtypes.CallToolRequest, error) {
	if action == nil {
		return nil, fmt.Errorf("action is nil")
	}
	name := strings.TrimSpace(action.Action)
	if name == "" {
		return nil, fmt.Errorf("action name is empty")
	}
	if IsControlAction(name) {
		return nil, fmt.Errorf("action %q is a control action, not a tool call", name)
	}
	if name == "call_tool" {
		return expandCallToolAction(action)
	}
	raw, err := snapshotReader(action.Data)
	if err != nil {
		return nil, err
	}
	action.Data = bytes.NewReader(raw)
	return []*streamtypes.CallToolRequest{{
		Index:     action.Index,
		Name:      name,
		Arguments: bytes.NewReader(raw),
		RawJSON:   strings.TrimSpace(action.RawJSON),
	}}, nil
}

func expandCallToolAction(action *streamtypes.ActionOutput) ([]*streamtypes.CallToolRequest, error) {
	if action == nil || action.Data == nil {
		return nil, fmt.Errorf("call_tool: missing data")
	}
	return []*streamtypes.CallToolRequest{{
		Index:     action.Index,
		Name:      "call_tool",
		Arguments: action.Data,
		RawJSON:   strings.TrimSpace(action.RawJSON),
	}}, nil
}

type callToolEntry struct {
	ToolCallID string
	ToolName   string
	ToolInput  map[string]interface{}
	Reason     string
}

func parseCallToolEntries(raw []byte) ([]callToolEntry, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("call_tool data: %w", err)
	}
	if rawCalls, ok := obj["tool_calls"].([]interface{}); ok && len(rawCalls) > 0 {
		out := make([]callToolEntry, 0, len(rawCalls))
		for _, item := range rawCalls {
			entry, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			name, input, callID, reason := resolveCallToolEntryFields(entry)
			if name == "" {
				continue
			}
			out = append(out, callToolEntry{ToolCallID: callID, ToolName: name, ToolInput: input, Reason: reason})
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	name, input, callID, reason := resolveCallToolEntryFields(obj)
	if name == "" {
		return nil, fmt.Errorf("call_tool: missing name")
	}
	return []callToolEntry{{ToolCallID: callID, ToolName: name, ToolInput: input, Reason: reason}}, nil
}

func resolveCallToolEntryFields(entry map[string]interface{}) (name string, input map[string]interface{}, callID string, reason string) {
	name, _ = entry["name"].(string)
	if strings.TrimSpace(name) == "" {
		name, _ = entry["tool_name"].(string)
	}
	name = strings.TrimSpace(name)
	callID, _ = entry["tool_call_id"].(string)
	reason, _ = entry["reason"].(string)
	switch {
	case entry["params"] != nil:
		if params, ok := entry["params"].(map[string]interface{}); ok {
			input = params
		}
	case entry["tool_input"] != nil:
		if toolInput, ok := entry["tool_input"].(map[string]interface{}); ok {
			input = toolInput
		}
	}
	if input == nil {
		input = map[string]interface{}{}
	}
	return name, input, strings.TrimSpace(callID), strings.TrimSpace(reason)
}

func snapshotReader(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, nil
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func mustRawJSON(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}

// DispatchParsedAction routes a parsed compatible envelope to tool calls or the control handler.
func DispatchParsedAction(action *streamtypes.ActionOutput, toolCalls chan<- *streamtypes.CallToolRequest, control streamtypes.CompatibleControlHandler) error {
	if action == nil {
		return fmt.Errorf("action is nil")
	}
	if IsControlAction(action.Action) {
		if control == nil {
			return fmt.Errorf("missing CompatibleControlHandler for action %q", action.Action)
		}
		return control(action)
	}
	reqs, err := ExpandCallToolRequests(action)
	if err != nil {
		return err
	}
	for _, req := range reqs {
		if req == nil {
			continue
		}
		toolCalls <- req
	}
	return nil
}
