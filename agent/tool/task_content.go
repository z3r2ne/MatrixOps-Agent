package tool

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"matrixops-agent/types"
	"pkgs/db/models"
	"pkgs/db/storage"

	"gorm.io/gorm"
)

type GetTaskContentTool struct {
	db     *gorm.DB
	taskID uint
}

func NewGetTaskContentTool(db *gorm.DB, taskID uint) *GetTaskContentTool {
	return &GetTaskContentTool{db: db, taskID: taskID}
}

func (GetTaskContentTool) Name() string {
	return "get_task_content"
}

func (GetTaskContentTool) VerbosName() string {
	return "获取任务内容"
}

func (GetTaskContentTool) Description() string {
	return "获取指定任务的 AI 对话记忆记录，返回序列化后的聊天记录字符串。可控制是否展示工具调用的输出内容。"
}

func (GetTaskContentTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"task_id": map[string]interface{}{
			"type":        "number",
			"description": "任务 ID，不提供则使用当前任务",
		},
		"include_tool_output": map[string]interface{}{
			"type":        "boolean",
			"description": "是否包含工具调用的输出内容，默认 true",
			"default":     true,
		},
	}, nil)
}

func (t *GetTaskContentTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	if t.db == nil {
		return Result{IsError: true}, errors.New("get_task_content: database unavailable")
	}

	taskID := t.taskID
	if rawTaskID, ok := input["task_id"]; ok {
		taskID = uint(intFrom(rawTaskID))
	}
	if taskID == 0 {
		return Result{IsError: true}, errors.New("get_task_content: task_id is required")
	}

	includeToolOutput := true
	if rawInclude, ok := input["include_tool_output"]; ok {
		includeToolOutput = boolFrom(rawInclude)
	}

	var task models.Task
	if err := t.db.First(&task, taskID).Error; err != nil {
		return Result{IsError: true}, fmt.Errorf("get_task_content: task not found: %w", err)
	}

	if strings.TrimSpace(task.SessionID) == "" {
		return Result{IsError: true}, errors.New("get_task_content: task has no session")
	}

	messages, err := storage.GetMessageWithPartsBySessionIDLight(t.db, task.SessionID)
	if err != nil {
		return Result{IsError: true}, fmt.Errorf("get_task_content: failed to load messages: %w", err)
	}

	content := serializeTaskMessages(messages, includeToolOutput)
	return Result{Content: content}, nil
}

func serializeTaskMessages(messages []*types.WithParts, includeToolOutput bool) string {
	var buf bytes.Buffer
	for _, msg := range messages {
		if msg.Info == nil {
			continue
		}
		buf.WriteString(fmt.Sprintf("## Message: %s\n", msg.Info.ID))
		buf.WriteString(fmt.Sprintf("- Role: %s\n", msg.Info.Role))
		if msg.Info.Worker != "" {
			buf.WriteString(fmt.Sprintf("- Worker: %s\n", msg.Info.Worker))
		}
		if msg.Info.Occupation != "" {
			buf.WriteString(fmt.Sprintf("- Occupation: %s\n", msg.Info.Occupation))
		}
		if msg.Info.Model != "" {
			buf.WriteString(fmt.Sprintf("- Model: %s\n", msg.Info.Model))
		}
		if msg.Info.Time.Created > 0 {
			buf.WriteString(fmt.Sprintf("- Created: %s\n", time.UnixMilli(msg.Info.Time.Created).Format(time.RFC3339)))
		}
		if msg.Info.Finish != "" {
			buf.WriteString(fmt.Sprintf("- Finish: %s\n", msg.Info.Finish))
		}
		if msg.Info.State != "" {
			buf.WriteString(fmt.Sprintf("- State: %s\n", msg.Info.State))
		}
		if msg.Info.Error != nil {
			buf.WriteString(fmt.Sprintf("- Error: %s\n", msg.Info.Error.Message))
		}
		buf.WriteString("\n")

		for _, part := range msg.Parts {
			serializePart(&buf, part, includeToolOutput)
		}
		buf.WriteString("\n---\n\n")
	}
	return buf.String()
}

func serializePart(buf *bytes.Buffer, part *types.Part, includeToolOutput bool) {
	if part == nil {
		return
	}
	buf.WriteString(fmt.Sprintf("### Part: %s (type: %s)\n", part.ID, part.Type))

	if part.AgentName != "" {
		buf.WriteString(fmt.Sprintf("- Agent: %s\n", part.AgentName))
	}
	if part.Description != "" {
		buf.WriteString(fmt.Sprintf("- Description: %s\n", part.Description))
	}

	switch part.Type {
	case types.PartTypeText, types.PartTypeTextDelta:
		if part.Text != "" {
			buf.WriteString("\n**Text:**\n")
			buf.WriteString(part.Text)
			buf.WriteString("\n")
		}
	case types.PartTypeReasoning, types.PartTypeReasoningDelta:
		if part.Reasoning != "" {
			buf.WriteString("\n**Reasoning:**\n")
			buf.WriteString(part.Reasoning)
			buf.WriteString("\n")
		}
	case types.PartTypeTool, types.PartTypeToolDelta:
		if part.Tool != nil {
			buf.WriteString(fmt.Sprintf("\n**Tool:** %s\n", part.Tool.Name))
			if part.Tool.CallID != "" {
				buf.WriteString(fmt.Sprintf("- CallID: %s\n", part.Tool.CallID))
			}
			if part.Tool.State.Status != "" {
				buf.WriteString(fmt.Sprintf("- Status: %s\n", part.Tool.State.Status))
			}
			if part.Tool.State.Input != nil {
				inputJSON, _ := json.MarshalIndent(part.Tool.State.Input, "", "  ")
				buf.WriteString(fmt.Sprintf("- Input:\n%s\n", string(inputJSON)))
			}
			if includeToolOutput {
				if part.Tool.State.Output != "" {
					buf.WriteString(fmt.Sprintf("- Output:\n%s\n", part.Tool.State.Output))
				}
				if part.Tool.State.FullOutput != "" && part.Tool.State.FullOutput != part.Tool.State.Output {
					buf.WriteString(fmt.Sprintf("- FullOutput:\n%s\n", part.Tool.State.FullOutput))
				}
			} else {
				buf.WriteString("- Output: [hidden]\n")
			}
			if part.Tool.State.Error != "" {
				buf.WriteString(fmt.Sprintf("- Error: %s\n", part.Tool.State.Error))
			}
		}
	case types.PartTypeError:
		if part.Error != nil {
			buf.WriteString(fmt.Sprintf("\n**Error:** %s - %s\n", part.Error.Name, part.Error.Message))
		}
	case types.PartTypeFinish:
		if part.Text != "" {
			buf.WriteString(fmt.Sprintf("\n**Finish:** %s\n", part.Text))
		}
	default:
		if part.Text != "" {
			buf.WriteString("\n**Text:**\n")
			buf.WriteString(part.Text)
			buf.WriteString("\n")
		}
	}

	if part.Metadata != nil && len(part.Metadata) > 0 {
		metaJSON, _ := json.MarshalIndent(part.Metadata, "", "  ")
		buf.WriteString(fmt.Sprintf("\n**Metadata:**\n%s\n", string(metaJSON)))
	}

	buf.WriteString("\n")
}
