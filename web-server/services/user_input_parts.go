package services

import (
	"fmt"
	"encoding/json"
	"strings"

	agenttypes "matrixops-agent/types"
	agentsession "matrixops-agent/session"
)

type wsUserPartWire struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Path     string `json:"path,omitempty"`
	URL      string `json:"url,omitempty"`
	Mime     string `json:"mime,omitempty"`
	Filename string `json:"filename,omitempty"`
	InputSource string `json:"inputSource,omitempty"`
	LegacySource string `json:"source,omitempty"`
}

// ParseWSUserParts 解析 WebSocket 用户消息 parts，并将 path 引用物化为 file:// Part。
func ParseWSUserParts(workDir string, taskID uint, raw json.RawMessage) ([]*agenttypes.Part, string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, "", nil
	}
	var wire []wsUserPartWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, "", fmt.Errorf("parts 格式错误: %w", err)
	}
	var textParts []string
	out := make([]*agenttypes.Part, 0, len(wire))
	for _, w := range wire {
		switch strings.TrimSpace(w.Type) {
		case "text":
			text := strings.TrimSpace(w.Text)
			if text == "" {
				continue
			}
			textParts = append(textParts, text)
			out = append(out, &agenttypes.Part{
				Type: agenttypes.PartTypeText,
				Text: text,
			})
		case "file":
			part := &agenttypes.Part{
				Type:     "file",
				Path:     strings.TrimSpace(w.Path),
				URL:      strings.TrimSpace(w.URL),
				Mime:     strings.TrimSpace(w.Mime),
				Filename: strings.TrimSpace(w.Filename),
				InputSource: strings.TrimSpace(firstNonEmpty(w.InputSource, w.LegacySource)),
			}
			if part.Path == "" && part.URL == "" {
				continue
			}
			if part.Path != "" {
				materialized, err := agentsession.MaterializeUserInputPart(workDir, part)
				if err != nil {
					return nil, "", fmt.Errorf("resolve file part %q: %w", part.Path, err)
				}
				materialized.URL = agentsession.TempUserInputFileAPIURL(materialized.Path)
				part = materialized
			}
			out = append(out, part)
		default:
			continue
		}
	}
	return out, strings.Join(textParts, "\n\n"), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
