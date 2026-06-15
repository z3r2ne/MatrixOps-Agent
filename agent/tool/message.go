package tool

import (
	"encoding/json"
	"fmt"
	"strings"
)

type MessageTool struct {
	deliver DeliverUserMessageFunc
}

func NewMessageTool(deliver DeliverUserMessageFunc) *MessageTool {
	return &MessageTool{deliver: deliver}
}

var _ Tool = (*MessageTool)(nil)

func (MessageTool) Name() string { return "message" }

func (MessageTool) VerbosName() string { return "发送消息" }

func (MessageTool) Description() string {
	return "向用户发送文本或媒体消息，可在任务执行中途通知用户或发送附件。"
}

func (MessageTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"text": map[string]interface{}{
			"type":        "string",
			"description": "文本内容",
		},
		"media": map[string]interface{}{
			"type":        "string",
			"description": "媒体文件路径或 URL",
		},
		"buffer": map[string]interface{}{
			"type":        "string",
			"description": "Base64 编码的文件内容",
		},
		"filePath": map[string]interface{}{
			"type":        "string",
			"description": "本地文件路径",
		},
		"filename": map[string]interface{}{
			"type":        "string",
			"description": "文件名",
		},
		"mimeType": map[string]interface{}{
			"type":        "string",
			"description": "MIME 类型",
		},
		"caption": map[string]interface{}{
			"type":        "string",
			"description": "媒体附带的文字说明",
		},
	}, nil)
}

func (t *MessageTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	if t.deliver == nil {
		return Result{IsError: true}, fmt.Errorf("message: 消息投递不可用")
	}
	params := UserDeliveryParams{
		Text:     strings.TrimSpace(toMessageString(input["text"])),
		Media:    strings.TrimSpace(toMessageString(input["media"])),
		Buffer:   strings.TrimSpace(toMessageString(input["buffer"])),
		FilePath: strings.TrimSpace(toMessageString(input["filePath"])),
		Filename: strings.TrimSpace(toMessageString(input["filename"])),
		MimeType: strings.TrimSpace(toMessageString(input["mimeType"])),
		Caption:  strings.TrimSpace(toMessageString(input["caption"])),
	}
	if params.Text == "" && params.Media == "" && params.Buffer == "" && params.FilePath == "" {
		return Result{IsError: true}, fmt.Errorf("message: 至少提供 text、media、buffer 或 filePath 之一")
	}
	if err := t.deliver(ctx, params); err != nil {
		return Result{IsError: true}, err
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"delivered": true,
		"text":      params.Text,
		"filename":  params.Filename,
	})
	return Result{Content: string(payload), Title: "已发送消息"}, nil
}

func toMessageString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", typed)
	}
}
