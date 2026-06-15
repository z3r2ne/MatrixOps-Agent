package coreagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"pkgs/ansi"
	"pkgs/jsonextractor"

	agenttool "matrixops-agent/tool"
	"matrixops.local/core_agent/streamtypes"
)

const DefaultStreamReaderInterval = 100 * time.Millisecond
const EmptyToolOutputPlaceholder = "[Tool Output]: 工具输出为空"

type v2ToolCallPayloadEntry struct {
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	ToolName   string                 `json:"name"`
	ToolInput  map[string]interface{} `json:"params"`
	Reason     string                 `json:"reason,omitempty"`
}

var CallToolActionSchema = ActionSchema{
	ActionName:  "call_tool",
	Description: CallToolActionSchemaDescription,
	DataSchema:  buildCallToolDataSchema(false),
}

const CallToolActionSchemaDescription = "调用一个或多个工具；当前轮结束后系统会继续把工具结果发回给你，进入下一轮循环"

func buildCallToolEntrySchema(requireReason bool) map[string]interface{} {
	properties := map[string]interface{}{
		"name": map[string]interface{}{
			"type":        "string",
			"description": "工具名称（见 <tools> 列表）",
		},
		"params": map[string]interface{}{
			"type":        "object",
			"description": "工具参数",
		},
	}
	required := []string{"name", "params"}
	if requireReason {
		properties["reason"] = map[string]interface{}{
			"type":        "string",
			"description": "调用该工具的原因",
		}
		required = append(required, "reason")
	}
	return map[string]interface{}{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

func buildCallToolDataSchema(requireReason bool) map[string]interface{} {
	entrySchema := buildCallToolEntrySchema(requireReason)
	return map[string]interface{}{
		"description": "调用一个或多个工具；工具结果会在后续循环中继续回传给你。",
		"oneOf": []interface{}{
			entrySchema,
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tool_calls": map[string]interface{}{
						"type":     "array",
						"items":    entrySchema,
						"minItems": 1,
					},
				},
				"required": []string{"tool_calls"},
			},
		},
	}
}

func (r *Runner) RegisterDefaultActions() {
	r.RegisterAction(NewActionHandler(MessageActionSchema, r.handleMessage))
	r.RegisterAction(NewActionHandler(AnswerActionSchema, r.handleAnswer))
	r.RegisterAction(NewActionHandler(CallToolActionSchema, r.handleCallTool))
}

var MessageActionSchema = ActionSchema{
	ActionName:  "message",
	Description: "向用户发送一条阶段性消息，但任务循环不会停止；系统随后仍会继续把新的工具结果或聊天记录发回给你",
	DataSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "发送给用户的阶段性消息内容",
			},
			"next_step": map[string]interface{}{
				"type":        "string",
				"description": "说明系统继续下一轮循环时你打算执行的下一步动作",
			},
		},
		"required":             []string{"message", "next_step"},
		"additionalProperties": false,
	},
}

var AnswerActionSchema = ActionSchema{
	ActionName:  "answer",
	Description: "回答用户并停止当前任务循环；仅在任务已完成或必须等待用户输入时使用",
	DataSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "最终回答内容。调用 answer 表示当前任务循环应停止。",
			},
		},
		"required":             []string{"content"},
		"additionalProperties": false,
	},
}

func buildSingleToolCallPayload(call v2ToolCallPayloadEntry) map[string]interface{} {
	data := map[string]interface{}{
		"name":   call.ToolName,
		"params": call.ToolInput,
	}
	if id := strings.TrimSpace(call.ToolCallID); id != "" {
		data["tool_call_id"] = id
	}
	if reason := strings.TrimSpace(call.Reason); reason != "" {
		data["reason"] = reason
	}
	return data
}

func PrepareActionDataReader(reader io.Reader) (io.Reader, byte, error) {
	var prefix bytes.Buffer
	buf := make([]byte, 1)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			prefix.WriteByte(buf[0])
			if !streamtypes.IsWhitespaceByte(buf[0]) {
				return io.MultiReader(bytes.NewReader(prefix.Bytes()), reader), buf[0], nil
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil, 0, io.ErrUnexpectedEOF
			}
			return nil, 0, err
		}
	}
}

func NormalizeTextContent(content string) string {
	return strings.Trim(strings.TrimSpace(content), `"`)
}

func formatToolInputPreview(arguments map[string]interface{}, raw string) string {
	if arguments != nil {
		if pretty, err := json.MarshalIndent(arguments, "", "  "); err == nil {
			return string(pretty)
		}
	}
	return strings.TrimSpace(raw)
}

func TryParseToolInput(content []byte) (map[string]interface{}, bool) {
	var input map[string]interface{}
	if err := json.Unmarshal(content, &input); err != nil {
		return nil, false
	}
	return input, true
}

func StreamJSONArrayObjects(reader io.Reader, onObject func(index int, reader io.Reader) error) error {
	var (
		itemWriter io.WriteCloser
		itemIndex  int
		depth      int
		inString   bool
		escaped    bool
		seenArray  bool
	)
	buffer := make([]byte, 1)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			ch := buffer[0]
			if !seenArray {
				if streamtypes.IsWhitespaceByte(ch) {
					if err == io.EOF {
						return io.ErrUnexpectedEOF
					}
					continue
				}
				if ch != '[' {
					return fmt.Errorf("expected array start, got %q", string(ch))
				}
				seenArray = true
				continue
			}
			if itemWriter == nil {
				if streamtypes.IsWhitespaceByte(ch) || ch == ',' {
					continue
				}
				if ch == ']' {
					return nil
				}
				if ch != '{' {
					return fmt.Errorf("expected array item object, got %q", string(ch))
				}
				itemReader, writer := jsonextractor.NewPipe()
				itemWriter = writer
				if onObject != nil {
					if err := onObject(itemIndex, itemReader); err != nil {
						_ = itemWriter.Close()
						itemWriter = nil
						return err
					}
				}
				depth = 0
				inString = false
				escaped = false
			}
			if _, writeErr := itemWriter.Write(buffer[:1]); writeErr != nil {
				return writeErr
			}
			if escaped {
				escaped = false
			} else {
				switch ch {
				case '\\':
					if inString {
						escaped = true
					}
				case '"':
					inString = !inString
				case '{':
					if !inString {
						depth++
					}
				case '}':
					if !inString {
						depth--
						if depth == 0 {
							if closeErr := itemWriter.Close(); closeErr != nil {
								return closeErr
							}
							itemWriter = nil
							itemIndex++
						}
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				if itemWriter != nil {
					_ = itemWriter.Close()
					return io.ErrUnexpectedEOF
				}
				return nil
			}
			return err
		}
	}
}

func ApplyToolCallParseError(part *Part, err error) {
	if part == nil || part.Tool == nil || err == nil {
		return
	}
	part.Tool.State.Status = "error"
	part.Tool.State.Error = err.Error()
	part.Tool.State.SystemMessage = "ERROR: call tool failed, reason: " + err.Error()
	part.Tool.State.Output = ""
	part.Tool.State.Time.End = time.Now().UnixMilli()
	if part.Tool.State.Metadata == nil {
		part.Tool.State.Metadata = map[string]interface{}{}
	}
	part.Tool.State.Metadata["toolError"] = err.Error()
}

func isToolExecutionCancelled(err error, result ToolResult, part *Part) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, agenttool.ErrToolExecutionCancelledByUser) {
		return true
	}
	if part != nil && part.Tool != nil {
		return hasToolResultCancelledBy("user", result.Metadata, part.Tool.State.Metadata) ||
			hasToolResultCancelledBy("stall_watchdog", result.Metadata, part.Tool.State.Metadata)
	}
	return hasToolResultCancelledBy("user", result.Metadata) ||
		hasToolResultCancelledBy("stall_watchdog", result.Metadata)
}

func ApplyToolCallExecution(part *Part, result ToolResult, err error) {
	if part == nil || part.Tool == nil {
		return
	}
	part.Tool.State.Time.End = time.Now().UnixMilli()
	mergeToolResultMetadataForCore(part, result)
	usesTerminalOutput := toolResultUsesTerminalOutput(part.Tool.State.Metadata, result.Metadata)
	if err != nil {
		if isToolExecutionCancelled(err, result, part) {
			part.Tool.State.Status = "cancelled"
			if toolResultCancelledByUser(result.Metadata, part.Tool.State.Metadata) {
				part.Tool.State.Error = "Tool execution was cancelled by user"
			} else {
				part.Tool.State.Error = "Tool execution cancelled"
			}
		} else {
			part.Tool.State.Status = "error"
			part.Tool.State.Error = err.Error()
		}
		systemMessage, body, _ := resolveToolResultBodyAndSystem(result, err)
		if usesTerminalOutput {
			if result.Content != "" {
				part.Tool.State.Output = ansi.StripTerminal(result.Content)
			}
		} else {
			part.Tool.State.SystemMessage = systemMessage
			part.Tool.State.Output = body
			if body == "" && systemMessage == "" {
				part.Tool.State.SystemMessage = "ERROR: " + err.Error()
			}
		}
		if part.Tool.State.Metadata == nil {
			part.Tool.State.Metadata = map[string]interface{}{}
		}
		part.Tool.State.Metadata["toolError"] = err.Error()
	} else {
		part.Tool.State.Status = "completed"
		systemMessage, body, _ := resolveToolResultBodyAndSystem(result, nil)
		if usesTerminalOutput {
			if result.Content != "" {
				part.Tool.State.Output = ansi.StripTerminal(result.Content)
			}
		} else {
			part.Tool.State.SystemMessage = systemMessage
			part.Tool.State.Output = body
		}
	}
	if result.Title != "" {
		part.Tool.State.Title = result.Title
	}
}

func (r *Runner) handleAnswer(ctx *ActionContext, action *ActionOutput) ([]*Part, error) {
	switch r.cfg.AnswerActionType {
	case ActionDataTypeJSONContent:
		var buf bytes.Buffer
		tee := io.TeeReader(action.Data, &buf)
		part := ctx.NewTextPart("answer")
		var (
			partMu      sync.Mutex
			contentSeen atomic.Bool
			handlerErr  error
			handlerDone = make(chan struct{})
		)
		setHandlerErr := func(err error) {
			if err == nil || handlerErr != nil {
				return
			}
			handlerErr = err
		}
		updatePart := func(content string) error {
			partMu.Lock()
			defer partMu.Unlock()
			part.Text = NormalizeTextContent(content)
			if part.Time != nil {
				part.Time.End = time.Now().UnixMilli()
			}
			return ctx.UpdatePart(part)
		}

		if err := jsonextractor.ExtractStructuredJSONFromStream(
			tee,
			jsonextractor.WithFormatKeyValueCallback(func(key, value any, parents []string) {
				if len(parents) != 0 {
					return
				}
				keyName, ok := key.(string)
				if !ok || keyName != "content" {
					return
				}
				contentSeen.Store(true)
			}),
			jsonextractor.WithRegisterFieldStreamHandler("content", func(key string, reader io.Reader, parents []string) {
				defer close(handlerDone)
				if len(parents) != 0 {
					_, _ = io.Copy(io.Discard, reader)
					return
				}
				contentSeen.Store(true)
				streamReader := NewStreamReader(jsonextractor.JSONStringReader(reader), DefaultStreamReaderInterval, updatePart)
				contentBytes, readErr := io.ReadAll(streamReader)
				if readErr != nil {
					setHandlerErr(fmt.Errorf("read answer content: %w", readErr))
					return
				}
				partMu.Lock()
				part.Text = NormalizeTextContent(string(contentBytes))
				if part.Time != nil && part.Time.End == 0 {
					part.Time.End = time.Now().UnixMilli()
				}
				partMu.Unlock()
			}),
		); err != nil {
			var payload map[string]interface{}
			if err2 := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &payload); err2 == nil {
				if content, ok := payload["content"].(string); ok && strings.TrimSpace(content) != "" {
					return buildMessageTextPart(ctx, content, "")
				}
			}
			return nil, fmt.Errorf("extract answer content: %w", err)
		}
		if contentSeen.Load() {
			<-handlerDone
		}
		if handlerErr != nil {
			return nil, handlerErr
		}
		if !contentSeen.Load() {
			return nil, fmt.Errorf("answer content is empty")
		}
		partMu.Lock()
		defer partMu.Unlock()
		return []*Part{part}, nil
	default:
		part, err := r.handleTextActionWithReader(ctx, action.Data, "answer")
		if err != nil {
			return nil, err
		}
		return []*Part{part}, nil
	}
}

// tryParseMessageToolArgs 解析 message 工具的完整 JSON 对象（原生 function 调用常见）。
// 部分紧凑对象在 jsonextractor 流式路径下可能无法触发 message 字段，导致 “message field is empty”。
func tryParseMessageToolArgs(raw []byte) (msg string, nextStep string, ok bool) {
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", "", false
	}
	msg = messageToolStringField(obj, "message")
	if msg == "" {
		msg = messageToolStringField(obj, "content")
	}
	nextStep = messageToolStringField(obj, "next_step")
	if msg == "" && nextStep != "" {
		msg = nextStep
	}
	if msg == "" {
		return "", "", false
	}
	return msg, nextStep, true
}

func isDeliveryMessageToolPayload(raw []byte) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return false
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	for _, key := range []string{"text", "filePath", "media", "buffer", "caption", "filename", "mimeType"} {
		if messageToolStringField(obj, key) != "" {
			return true
		}
	}
	return false
}

func snapshotActionData(action *ActionOutput) ([]byte, error) {
	if action == nil || action.Data == nil {
		return nil, nil
	}
	data, err := io.ReadAll(action.Data)
	if err != nil {
		return nil, err
	}
	action.Data = io.NopCloser(bytes.NewReader(data))
	return data, nil
}

func (r *Runner) tryHandleDeliveryMessageTool(ctx *ActionContext, action *ActionOutput, raw []byte) ([]*Part, error) {
	if !isDeliveryMessageToolPayload(raw) {
		return nil, fmt.Errorf("message field is empty")
	}
	if r.tools == nil {
		return nil, fmt.Errorf("message field is empty")
	}
	if _, err := r.tools.Get("message"); err != nil {
		return nil, fmt.Errorf("message field is empty")
	}
	actionCopy := *action
	actionCopy.Data = io.NopCloser(bytes.NewReader(bytes.TrimSpace(raw)))
	return r.handleDirectRegistryTool(ctx, "message", &actionCopy)
}

func buildMessageTextPart(ctx *ActionContext, msg, nextStep string) ([]*Part, error) {
	part := ctx.NewTextPart("message")
	part.Text = NormalizeTextContent(msg)
	if nextStep != "" {
		if part.Metadata == nil {
			part.Metadata = map[string]interface{}{}
		}
		part.Metadata["next_step"] = nextStep
	}
	if part.Time != nil && part.Time.End == 0 {
		part.Time.End = time.Now().UnixMilli()
	}
	if err := ctx.UpdatePart(part); err != nil {
		return nil, err
	}
	return []*Part{part}, nil
}

func messageToolStringField(obj map[string]interface{}, key string) string {
	v, exists := obj[key]
	if !exists || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return strings.TrimSpace(t.String())
	case float64:
		return strings.TrimSpace(fmt.Sprint(t))
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func (r *Runner) handleCallTool(ctx *ActionContext, action *ActionOutput) ([]*Part, error) {
	raw, err := snapshotActionData(action)
	if err != nil {
		return nil, fmt.Errorf("read call_tool data: %w", err)
	}
	calls, err := parseCallToolEntries(raw)
	if err != nil {
		return nil, err
	}
	if len(calls) == 0 {
		return nil, fmt.Errorf("call_tool: no tool calls")
	}
	parts := make([]*Part, 0, len(calls))
	var partsMu sync.Mutex
	err = runIndexedParallel(len(calls), MaxConcurrentToolCalls, func(index int) error {
		call := calls[index]
		payload, err := json.Marshal(call.ToolInput)
		if err != nil {
			return fmt.Errorf("marshal tool params: %w", err)
		}
		actionCopy := *action
		actionCopy.Action = call.ToolName
		actionCopy.Data = bytes.NewReader(payload)
		toolParts, err := r.handleDirectRegistryTool(ctx, call.ToolName, &actionCopy)
		if err != nil {
			return err
		}
		partsMu.Lock()
		parts = append(parts, toolParts...)
		partsMu.Unlock()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return parts, nil
}

type callToolEntry struct {
	ToolName  string
	ToolInput map[string]interface{}
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
			name, input := resolveCallToolEntryFields(entry)
			if name == "" {
				continue
			}
			out = append(out, callToolEntry{ToolName: name, ToolInput: input})
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	name, input := resolveCallToolEntryFields(obj)
	if name == "" {
		return nil, fmt.Errorf("call_tool: missing name")
	}
	return []callToolEntry{{ToolName: name, ToolInput: input}}, nil
}

func resolveCallToolEntryFields(entry map[string]interface{}) (string, map[string]interface{}) {
	name, _ := entry["name"].(string)
	if strings.TrimSpace(name) == "" {
		name, _ = entry["tool_name"].(string)
	}
	name = strings.TrimSpace(name)
	var input map[string]interface{}
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
	return name, input
}

func (r *Runner) handleMessage(ctx *ActionContext, action *ActionOutput) ([]*Part, error) {
	var buf bytes.Buffer
	tee := io.TeeReader(action.Data, &buf)
	preparedReader, firstByte, err := PrepareActionDataReader(tee)
	if err != nil {
		return nil, fmt.Errorf("prepare message content: %w", err)
	}
	if firstByte == '"' {
		part, err := r.handleTextActionWithReader(ctx, preparedReader, "message")
		if err != nil {
			return nil, err
		}
		return []*Part{part}, nil
	}
	if firstByte != '{' {
		part, err := r.handleTextActionWithReader(ctx, preparedReader, "message")
		if err != nil {
			return nil, err
		}
		return []*Part{part}, nil
	}

	part := ctx.NewTextPart("message")
	var (
		partMu      sync.Mutex
		nextStep    string
		messageSeen atomic.Bool
		handlerErr  error
		handlerDone = make(chan struct{})
	)
	setHandlerErr := func(err error) {
		if err == nil || handlerErr != nil {
			return
		}
		handlerErr = err
	}
	updatePart := func(content string) error {
		partMu.Lock()
		defer partMu.Unlock()
		part.Text = NormalizeTextContent(content)
		if strings.TrimSpace(nextStep) != "" {
			part.Metadata["next_step"] = strings.TrimSpace(nextStep)
		}
		if part.Time != nil {
			part.Time.End = time.Now().UnixMilli()
		}
		return ctx.UpdatePart(part)
	}
	if err := jsonextractor.ExtractStructuredJSONFromStream(
		preparedReader,
		jsonextractor.WithFormatKeyValueCallback(func(key, value any, parents []string) {
			if len(parents) != 0 {
				return
			}
			keyName, ok := key.(string)
			if !ok {
				return
			}
			switch keyName {
			case "next_step":
				if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
					partMu.Lock()
					nextStep = text
					partMu.Unlock()
				}
			case "message":
				// 字段流 handler 在单独 goroutine 中运行；主 goroutine 可能在 handler 置位前就结束解析。
				// 此处只同步置位 messageSeen；正文与时间戳仍由流式 handler /最终 return 路径写入，避免与 updatePart 竞态。
				messageSeen.Store(true)
			default:
				return
			}
		}),
		jsonextractor.WithRegisterFieldStreamHandler("message", func(key string, reader io.Reader, parents []string) {
			defer close(handlerDone)
			if len(parents) != 0 {
				_, _ = io.Copy(io.Discard, reader)
				return
			}
			messageSeen.Store(true)
			streamReader := NewStreamReader(jsonextractor.JSONStringReader(reader), DefaultStreamReaderInterval, updatePart)
			contentBytes, readErr := io.ReadAll(streamReader)
			if readErr != nil {
				setHandlerErr(fmt.Errorf("read message content: %w", readErr))
				return
			}
			partMu.Lock()
			part.Text = NormalizeTextContent(string(contentBytes))
			if strings.TrimSpace(nextStep) != "" {
				part.Metadata["next_step"] = strings.TrimSpace(nextStep)
			}
			if part.Time != nil && part.Time.End == 0 {
				part.Time.End = time.Now().UnixMilli()
			}
			partMu.Unlock()
		}),
	); err != nil {
		if msg, ns, ok := tryParseMessageToolArgs(bytes.TrimSpace(buf.Bytes())); ok {
			return buildMessageTextPart(ctx, msg, ns)
		}
		if parts, err := r.tryHandleDeliveryMessageTool(ctx, action, buf.Bytes()); err == nil {
			return parts, nil
		}
		return nil, fmt.Errorf("extract message content: %w", err)
	}
	if messageSeen.Load() {
		<-handlerDone
	}
	if handlerErr != nil {
		if msg, ns, ok := tryParseMessageToolArgs(bytes.TrimSpace(buf.Bytes())); ok {
			return buildMessageTextPart(ctx, msg, ns)
		}
		if parts, err := r.tryHandleDeliveryMessageTool(ctx, action, buf.Bytes()); err == nil {
			return parts, nil
		}
		return nil, handlerErr
	}
	if !messageSeen.Load() {
		if msg, ns, ok := tryParseMessageToolArgs(bytes.TrimSpace(buf.Bytes())); ok {
			return buildMessageTextPart(ctx, msg, ns)
		}
		return r.tryHandleDeliveryMessageTool(ctx, action, buf.Bytes())
	}
	partMu.Lock()
	defer partMu.Unlock()
	if strings.TrimSpace(nextStep) != "" {
		part.Metadata["next_step"] = strings.TrimSpace(nextStep)
	}
	return []*Part{part}, nil
}

func (r *Runner) handleTextAction(ctx *ActionContext, action *ActionOutput, actionName string) (*Part, error) {
	preparedReader, firstByte, err := PrepareActionDataReader(action.Data)
	if err != nil {
		return nil, fmt.Errorf("prepare %s content: %w", actionName, err)
	}
	if firstByte == '"' {
		return r.handleTextActionWithReader(ctx, jsonextractor.JSONStringReader(preparedReader), actionName)
	}
	return r.handleTextActionWithReader(ctx, preparedReader, actionName)
}

func (r *Runner) handleTextActionWithReader(ctx *ActionContext, reader io.Reader, actionName string) (*Part, error) {
	textPart := ctx.NewTextPart(actionName)
	var textPartMu sync.Mutex
	streamReader := NewStreamReader(reader, DefaultStreamReaderInterval, func(content string) error {
		textPartMu.Lock()
		defer textPartMu.Unlock()
		textPart.Text = NormalizeTextContent(content)
		if textPart.Time != nil {
			textPart.Time.End = time.Now().UnixMilli()
		}
		return ctx.UpdatePart(textPart)
	})
	contentBytes, err := io.ReadAll(streamReader)
	if err != nil {
		return nil, fmt.Errorf("read %s content: %w", actionName, err)
	}
	textPartMu.Lock()
	defer textPartMu.Unlock()
	textPart.Text = NormalizeTextContent(string(contentBytes))
	if textPart.Time != nil && textPart.Time.End == 0 {
		textPart.Time.End = time.Now().UnixMilli()
	}
	return textPart, nil
}

func (r *Runner) handleDirectRegistryTool(ctx *ActionContext, toolName string, action *ActionOutput) ([]*Part, error) {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return nil, fmt.Errorf("tool name is empty")
	}

	callID := r.nextID("tool")
	toolExecCtx, cancelTool, cleanupTool := agenttool.DeriveToolCallContext(ctx.Context, callID, toolName)
	defer cleanupTool()

	part := ctx.NewToolPart(toolName)
	var partMu sync.Mutex
	updateToolPart := func(mutate func()) error {
		partMu.Lock()
		defer partMu.Unlock()
		if mutate != nil {
			mutate()
		}
		return ctx.UpdatePart(part)
	}
	part.Tool.CallID = callID
	ctx.SetToolPart(callID, part)
	if err := updateToolPart(func() {
		part.Tool.State.Status = "preparing"
		part.Tool.State.Metadata = map[string]interface{}{
			"inputPreview":   "",
			"inputStreaming": true,
			"cancelable":     true,
		}
	}); err != nil {
		return nil, err
	}

	streamReader := NewStreamReader(action.Data, DefaultStreamReaderInterval, func(content string) error {
		return updateToolPart(func() {
			part.Tool.State.Status = "input-streaming"
			if part.Tool.State.Metadata == nil {
				part.Tool.State.Metadata = map[string]interface{}{}
			}
			part.Tool.State.Metadata["inputPreview"] = strings.TrimSpace(content)
			part.Tool.State.Metadata["inputStreaming"] = true
		})
	})
	payload, err := io.ReadAll(streamReader)
	if toolExecCtx.Err() != nil {
		execErr := context.Cause(toolExecCtx)
		if execErr == nil {
			execErr = context.Canceled
		}
		result := ToolResult{
			IsError: true,
			Name:    toolName,
			Content: "[Tool Cancelled]: tool execution was cancelled by user",
			Metadata: map[string]interface{}{
				"cancelled":   true,
				"cancelledBy": "user",
			},
		}
		_ = updateToolPart(func() {
			ApplyToolCallExecution(part, result, execErr)
		})
		return []*Part{part}, nil
	}
	if err != nil {
		toolErr := fmt.Errorf("read tool arguments: %w", err)
		if updateErr := updateToolPart(func() {
			ApplyToolCallParseError(part, toolErr)
		}); updateErr != nil {
			return nil, updateErr
		}
		return []*Part{part}, nil
	}
	var args map[string]interface{}
	if len(bytes.TrimSpace(payload)) == 0 {
		args = map[string]interface{}{}
	} else if err := json.Unmarshal(payload, &args); err != nil {
		toolErr := fmt.Errorf("tool %q arguments: %w", toolName, err)
		if updateErr := updateToolPart(func() {
			part.Tool.State.Raw = strings.TrimSpace(string(payload))
			if part.Tool.State.Metadata == nil {
				part.Tool.State.Metadata = map[string]interface{}{}
			}
			part.Tool.State.Metadata["inputPreview"] = strings.TrimSpace(string(payload))
			part.Tool.State.Metadata["inputStreaming"] = false
			part.Tool.State.Metadata["cancelable"] = false
			ApplyToolCallParseError(part, toolErr)
		}); updateErr != nil {
			return nil, updateErr
		}
		return []*Part{part}, nil
	}
	if err := updateToolPart(func() {
		part.Tool.State.Input = args
		part.Tool.State.Raw = strings.TrimSpace(string(payload))
		part.Tool.State.Status = "running"
		part.Tool.State.Metadata = map[string]interface{}{
			"inputPreview":   strings.TrimSpace(string(payload)),
			"inputStreaming": false,
			"cancelable":     true,
		}
	}); err != nil {
		return nil, err
	}
	call := ToolCall{ID: callID, Name: toolName, Arguments: args}

	watchdogTimeout := ctx.State.ResolveStallWatchdogTimeout(toolName, r.cfg.StallWatchdogTimeout)
	if watchdogTimeout <= 0 {
		result, execErr := r.executeToolCallWithContext(ctx, call, toolExecCtx)
		_ = updateToolPart(func() {
			ApplyToolCallExecution(part, result, execErr)
		})
		return []*Part{part}, nil
	}

	type toolExecResult struct {
		result ToolResult
		err    error
	}
	done := make(chan toolExecResult, 1)
	go func() {
		res, err := r.executeToolCallWithContext(ctx, call, toolExecCtx)
		done <- toolExecResult{result: res, err: err}
	}()

	startTime := r.now()
	var result ToolResult
	var execErr error

	timer := time.NewTimer(watchdogTimeout)
	defer timer.Stop()

	toolRunning := true
	for toolRunning {
		select {
		case res := <-done:
			result = res.result
			execErr = res.err
			toolRunning = false
		case <-toolExecCtx.Done():
			cancelTool(agenttool.ErrToolExecutionCancelledByUser)
			execErr = context.Cause(toolExecCtx)
			if execErr == nil {
				execErr = context.Canceled
			}
			result = ToolResult{
				IsError: true,
				Name:    toolName,
				Content: "[Tool Cancelled]: tool execution was cancelled by user",
				Metadata: map[string]interface{}{
					"cancelled":   true,
					"cancelledBy": "user",
				},
			}
			select {
			case res := <-done:
				if res.err != nil {
					execErr = res.err
				}
				if res.result.Name != "" || res.result.Content != "" {
					result = res.result
				}
			case <-time.After(2 * time.Second):
			}
			toolRunning = false
		case <-ctx.Context.Done():
			cancelTool(agenttool.ErrToolExecutionCancelledByUser)
			execErr = context.Cause(ctx.Context)
			if execErr == nil {
				execErr = context.Canceled
			}
			result = ToolResult{
				IsError: true,
				Name:    toolName,
				Content: "[Tool Cancelled]: tool execution was cancelled by user",
				Metadata: map[string]interface{}{
					"cancelled":   true,
					"cancelledBy": "user",
				},
			}
			select {
			case res := <-done:
				if res.err != nil {
					execErr = res.err
				}
				if res.result.Name != "" || res.result.Content != "" {
					result = res.result
				}
			case <-time.After(2 * time.Second):
			}
			toolRunning = false
		case <-timer.C:
			elapsed := r.now().Sub(startTime)
			currentOutput := part.Tool.State.Output
			r.emitAssistantFooterStatus(ctx.State.Assistant.ID, "工具执行停滞，询问模型…", true)
			decision, reason, continueWait, resolveErr := r.resolveStall(ctx.State, toolName, args, currentOutput, elapsed)
			if resolveErr != nil {
				r.emitAssistantFooterStatus(ctx.State.Assistant.ID, "询问模型失败，继续等待工具执行…", true)
				timer.Reset(watchdogTimeout)
				continue
			}
			if decision == "cancel" {
				r.emitAssistantFooterStatus(ctx.State.Assistant.ID, "工具执行已取消: "+reason, false)
				cancelTool(fmt.Errorf("stall watchdog: cancelled by model decision: %s", reason))
				select {
				case res := <-done:
					result = res.result
					execErr = res.err
					ensureStallWatchdogCancelMetadata(&result, toolName, reason, elapsed)
				case <-time.After(5 * time.Second):
					result = ToolResult{
						IsError: true,
						Name:    toolName,
						Content: fmt.Sprintf("[Tool Cancelled]: tool %q was cancelled by stall watchdog after %v: %s", toolName, elapsed, reason),
						Metadata: map[string]interface{}{
							"cancelled":      true,
							"cancelledBy":    "stall_watchdog",
							"watchdogReason": reason,
						},
					}
					execErr = fmt.Errorf("stall watchdog: tool execution cancelled after timeout")
				}
				toolRunning = false
				r.notifyStallWatchdogToolCancelled(ctx.State, toolName, callID, reason, elapsed)
			} else {
				r.emitAssistantFooterStatus(ctx.State.Assistant.ID, fmt.Sprintf("模型决定继续等待 %v: %s", continueWait, reason), true)
				timer.Reset(continueWait)
			}
		}
	}

	_ = updateToolPart(func() {
		ApplyToolCallExecution(part, result, execErr)
	})
	// 工具执行失败（含非零退出、取消）只记录在 tool part，不终止 agent 循环。
	return []*Part{part}, nil
}

func toolResultUsesTerminalOutput(metadata ...map[string]interface{}) bool {
	for _, item := range metadata {
		if len(item) == 0 {
			continue
		}
		if format, ok := item["outputFormat"].(string); ok && format == "terminal" {
			return true
		}
		if streamMode, ok := item["streamMode"].(string); ok && streamMode == "terminal" {
			return true
		}
		if tty, ok := item["tty"].(bool); ok && tty {
			return true
		}
	}
	return false
}

func toolResultCancelledByUser(metadata ...map[string]interface{}) bool {
	return hasToolResultCancelledBy("user", metadata...)
}

func hasToolResultCancelledBy(cancelledBy string, metadata ...map[string]interface{}) bool {
	for _, item := range metadata {
		if len(item) == 0 {
			continue
		}
		if by, ok := item["cancelledBy"].(string); ok && by == cancelledBy {
			return true
		}
	}
	return false
}

func ensureStallWatchdogCancelMetadata(result *ToolResult, toolName, reason string, elapsed time.Duration) {
	if result == nil {
		return
	}
	if hasToolResultCancelledBy("stall_watchdog", result.Metadata) {
		return
	}
	if result.Metadata == nil {
		result.Metadata = map[string]interface{}{}
	}
	result.IsError = true
	if result.Name == "" {
		result.Name = toolName
	}
	if strings.TrimSpace(result.Content) == "" {
		result.Content = fmt.Sprintf(
			"[Tool Cancelled]: tool %q was cancelled by stall watchdog after %v: %s",
			toolName,
			elapsed,
			reason,
		)
	}
	result.Metadata["cancelled"] = true
	result.Metadata["cancelledBy"] = "stall_watchdog"
	result.Metadata["watchdogReason"] = reason
}
