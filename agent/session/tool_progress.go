package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	coreagent "matrixops.local/core_agent"

	"matrixops-agent/tool"
)

const (
	toolOutputFormatKey      = "outputFormat"
	toolOutputFormatTerminal = "terminal"
	maxLiveToolOutputBytes   = 200_000
)

func newSessionToolProgressReporter(r *AgentRunner, part *Part) func(tool.StreamEvent) {
	var mu sync.Mutex
	return func(event tool.StreamEvent) {
		if r == nil || part == nil || part.Tool == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		applyToolStreamEventToSessionPart(part, event)
		if part.Tool.State.Time.End == 0 {
			part.Tool.State.Time.End = time.Now().UnixMilli()
		}
		_, _ = r.emitter.UpdatePart(part)
	}
}

func newCoreToolProgressReporter(actionCtx *coreagent.ActionContext, part *coreagent.Part) func(tool.StreamEvent) {
	var mu sync.Mutex
	return func(event tool.StreamEvent) {
		if actionCtx == nil || part == nil || part.Tool == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		applyToolStreamEventToCorePart(part, event)
		if part.Tool.State.Time.End == 0 {
			part.Tool.State.Time.End = time.Now().UnixMilli()
		}
		_ = actionCtx.UpdatePart(part)
	}
}

func applyToolStreamEventToSessionPart(part *Part, event tool.StreamEvent) {
	if part == nil || part.Tool == nil {
		return
	}
	applyToolStreamState(&part.Tool.State, event)
}

func applyToolStreamEventToCorePart(part *coreagent.Part, event tool.StreamEvent) {
	if part == nil || part.Tool == nil {
		return
	}
	applyCoreToolStreamState(&part.Tool.State, event)
}

func applyToolStreamState(state *ToolState, event tool.StreamEvent) {
	if state == nil {
		return
	}
	if strings.TrimSpace(event.Status) != "" {
		state.Status = strings.TrimSpace(event.Status)
	}
	if strings.TrimSpace(event.Title) != "" {
		state.Title = strings.TrimSpace(event.Title)
	}
	if state.Metadata == nil {
		state.Metadata = map[string]interface{}{}
	}
	mergeToolStreamMetadata(state.Metadata, event.Metadata)
	if strings.TrimSpace(event.Content) != "" {
		state.Output = appendLiveToolOutput(state.Output, event.Content)
		state.Metadata[toolOutputFormatKey] = toolOutputFormatTerminal
	}
}

func applyCoreToolStreamState(state *coreagent.ToolState, event tool.StreamEvent) {
	if state == nil {
		return
	}
	if strings.TrimSpace(event.Status) != "" {
		state.Status = strings.TrimSpace(event.Status)
	}
	if strings.TrimSpace(event.Title) != "" {
		state.Title = strings.TrimSpace(event.Title)
	}
	if state.Metadata == nil {
		state.Metadata = map[string]interface{}{}
	}
	mergeToolStreamMetadata(state.Metadata, event.Metadata)
	if strings.TrimSpace(event.Content) != "" {
		state.Output = appendLiveToolOutput(state.Output, event.Content)
		state.Metadata[toolOutputFormatKey] = toolOutputFormatTerminal
	}
}

func mergeToolStreamMetadata(target map[string]interface{}, source map[string]interface{}) {
	if len(source) == 0 {
		return
	}
	for key, value := range source {
		target[key] = value
	}
}

func appendLiveToolOutput(current string, chunk string) string {
	if chunk == "" {
		return current
	}
	current += chunk
	if len(current) <= maxLiveToolOutputBytes {
		return current
	}
	tail := current[len(current)-maxLiveToolOutputBytes:]
	marker := "\r\n...[terminal output truncated]...\r\n"
	if len(marker)+len(tail) > maxLiveToolOutputBytes {
		tail = tail[len(marker)+len(tail)-maxLiveToolOutputBytes:]
	}
	return marker + tail
}

func toolUsesTerminalOutput(metadata map[string]interface{}) bool {
	if len(metadata) == 0 {
		return false
	}
	if format, ok := metadata[toolOutputFormatKey].(string); ok && format == toolOutputFormatTerminal {
		return true
	}
	if streamMode, ok := metadata["streamMode"].(string); ok && streamMode == "terminal" {
		return true
	}
	if tty, ok := metadata["tty"].(bool); ok && tty {
		return true
	}
	return false
}

func isToolExecutionCancelled(err error) bool {
	return errors.Is(err, context.Canceled)
}

func toolCancelledByUser(metadata map[string]interface{}) bool {
	if len(metadata) == 0 {
		return false
	}
	cancelledBy, _ := metadata["cancelledBy"].(string)
	return cancelledBy == "user"
}

func buildToolCallResultMessage(result tool.Result, err error) string {
	if result.Metadata != nil {
		if cancelled, _ := result.Metadata["cancelled"].(bool); cancelled {
			lines := make([]string, 0, 2)
			if content := strings.TrimSpace(result.Content); content != "" {
				if toolUsesTerminalOutput(result.Metadata) {
					lines = append(lines, "[Tool Partial Output]:\n"+content)
				} else {
					lines = append(lines, fmt.Sprintf("[Tool Partial Output]: %s", content))
				}
			}
			if toolCancelledByUser(result.Metadata) {
				lines = append(lines, "[Tool Cancelled]: tool execution was cancelled by user during execution")
			} else {
				lines = append(lines, "[Tool Cancelled]: tool execution was cancelled during execution")
			}
			return strings.Join(lines, "\n")
		}
	}
	if err != nil {
		return fmt.Sprintf("[Tool Error]: %s", err.Error())
	}
	if result.Content != "" {
		return fmt.Sprintf("[Tool Output]: %s", result.Content)
	}
	return ""
}
