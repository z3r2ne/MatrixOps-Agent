package session

import (
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"matrixops-agent/tool"
	"matrixops-agent/types"
	"matrixops-agent/util"
	"pkgs/db/storage"

	"gorm.io/gorm"
)

// DeliverUserMessage 向当前会话投递一条 assistant 消息（文本 + 可选附件），返回新建消息 ID。
func DeliverUserMessage(db *gorm.DB, emitter *Emitter, workDir string, params tool.UserDeliveryParams) (string, error) {
	if emitter == nil || strings.TrimSpace(emitter.SessionID) == "" {
		return "", fmt.Errorf("message: 会话不可用")
	}
	text := composeDeliveryText(params)
	filePart, err := buildDeliveryFilePart(workDir, params)
	if err != nil {
		return "", err
	}
	if text == "" && filePart == nil {
		return "", fmt.Errorf("message: 没有可发送的内容")
	}

	now := time.Now().UnixMilli()
	messageID := util.Ascending("message")
	info := &types.MessageInfo{
		ID:        messageID,
		SessionID: emitter.SessionID,
		Role:      types.RoleAssistant,
		Finish:    "stop",
		Time: types.MessageTime{
			Created:   now,
			Completed: now,
		},
	}
	if db != nil {
		if err := storage.UpdateMessageInfo(db, info); err != nil {
			return "", err
		}
	}

	parts := make([]*types.Part, 0, 2)
	if text != "" {
		parts = append(parts, &types.Part{
			ID:        util.Ascending("part"),
			MessageID: messageID,
			SessionID: emitter.SessionID,
			Type:      types.PartTypeText,
			Text:      text,
			Time:      &types.PartTime{Created: now, Start: now, End: now},
		})
	}
	if filePart != nil {
		filePart.MessageID = messageID
		filePart.SessionID = emitter.SessionID
		if filePart.ID == "" {
			filePart.ID = util.Ascending("part")
		}
		if filePart.Time == nil {
			filePart.Time = &types.PartTime{Created: now, Start: now, End: now}
		}
		parts = append(parts, filePart)
	}

	// 先将所有 part 写入 DB，避免 UpdatePart 触发的转发逻辑在附件尚未落库时只看到文本。
	for _, part := range parts {
		if db != nil {
			if _, err := storage.UpdatePart(db, part); err != nil {
				return "", err
			}
		}
	}
	for _, part := range parts {
		if _, err := emitter.UpdatePart(part); err != nil {
			return "", err
		}
	}
	if _, err := emitter.UpdateMessage(info); err != nil {
		return "", err
	}
	return messageID, nil
}

func composeDeliveryText(params tool.UserDeliveryParams) string {
	text := strings.TrimSpace(params.Text)
	caption := strings.TrimSpace(params.Caption)
	switch {
	case text != "" && caption != "" && text != caption:
		return text + "\n\n" + caption
	case text != "":
		return text
	default:
		return caption
	}
}

func buildDeliveryFilePart(workDir string, params tool.UserDeliveryParams) (*types.Part, error) {
	url, filename, mimeType, err := resolveDeliveryAttachment(workDir, params)
	if err != nil {
		return nil, err
	}
	if url == "" {
		return nil, nil
	}
	if filename == "" {
		filename = "attachment"
	}
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(filename))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}
	return &types.Part{
		Type:     "file",
		URL:      url,
		Filename: filename,
		Mime:     mimeType,
	}, nil
}

func resolveDeliveryAttachment(workDir string, params tool.UserDeliveryParams) (url, filename, mimeType string, err error) {
	filename = strings.TrimSpace(params.Filename)
	mimeType = strings.TrimSpace(params.MimeType)

	if buffer := strings.TrimSpace(params.Buffer); buffer != "" {
		data, decodeErr := base64.StdEncoding.DecodeString(buffer)
		if decodeErr != nil {
			return "", "", "", fmt.Errorf("message: buffer Base64 无效")
		}
		if filename == "" {
			filename = "attachment.bin"
		}
		if mimeType == "" {
			mimeType = http.DetectContentType(data)
		}
		return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), filename, mimeType, nil
	}

	candidates := []string{
		strings.TrimSpace(params.FilePath),
		strings.TrimSpace(params.Media),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if strings.HasPrefix(candidate, "http://") || strings.HasPrefix(candidate, "https://") || strings.HasPrefix(candidate, "data:") {
			if filename == "" {
				filename = filepath.Base(candidate)
			}
			return candidate, filename, mimeType, nil
		}
		path := candidate
		if !filepath.IsAbs(path) && strings.TrimSpace(workDir) != "" {
			path = filepath.Join(workDir, path)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return "", "", "", fmt.Errorf("message: 读取文件失败 %s: %w", candidate, readErr)
		}
		if filename == "" {
			filename = filepath.Base(path)
		}
		if mimeType == "" {
			mimeType = mime.TypeByExtension(filepath.Ext(path))
			if mimeType == "" {
				mimeType = http.DetectContentType(data)
			}
		}
		return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), filename, mimeType, nil
	}
	return "", "", "", nil
}
