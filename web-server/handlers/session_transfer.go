package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"matrixops-agent/types"
	"matrixops-agent/util"
	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const sessionTransferKind = "matrixops-session-transfer"
const sessionTransferVersion = 1

type SessionTransferPayload struct {
	Kind            string               `json:"kind"`
	Version         int                  `json:"version"`
	ExportedAt      int64                `json:"exportedAt"`
	SourceSessionID string               `json:"sourceSessionId,omitempty"`
	Session         *types.Info          `json:"session,omitempty"`
	Messages        []*types.WithParts   `json:"messages"`
	MemoryEntries   []*types.MemoryEntry `json:"memoryEntries"`
}

func (h *SessionHandler) ExportSessionTransfer(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}

	session, err := storage.GetSession(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	messages, err := storage.GetMessageWithPartsBySessionID(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取聊天记录失败"})
		return
	}

	memoryEntries, err := storage.ListMemoryEntriesBySession(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取历史记忆失败"})
		return
	}

	payload := SessionTransferPayload{
		Kind:            sessionTransferKind,
		Version:         sessionTransferVersion,
		ExportedAt:      time.Now().UnixMilli(),
		SourceSessionID: sessionID,
		Session:         session,
		Messages:        messages,
		MemoryEntries:   memoryEntries,
	}

	// 构造 ZIP
	zipBuffer, filename, err := buildSessionTransferZip(h.db, payload, session.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("构造导出文件失败: %v", err)})
		return
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "application/zip", zipBuffer.Bytes())
}

func buildSessionTransferZip(db *gorm.DB, payload SessionTransferPayload, title string) (*bytes.Buffer, string, error) {
	safeTitle := sanitizeExportFilename(title)
	if safeTitle == "" {
		safeTitle = "session"
	}
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	filename := fmt.Sprintf("%s-memory-chat-%s.zip", safeTitle, timestamp)

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	defer zw.Close()

	// 1. data.json
	jsonData, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("marshal json: %w", err)
	}
	w, err := zw.Create(fmt.Sprintf("%s-data.json", safeTitle))
	if err != nil {
		return nil, "", fmt.Errorf("create json entry: %w", err)
	}
	if _, err := w.Write(jsonData); err != nil {
		return nil, "", fmt.Errorf("write json: %w", err)
	}

	// 2. chat.txt (XML格式，按记忆导出)
	txtContent := serializeMemoriesToXML(payload)
	w, err = zw.Create(fmt.Sprintf("%s-chat.txt", safeTitle))
	if err != nil {
		return nil, "", fmt.Errorf("create txt entry: %w", err)
	}
	if _, err := w.Write([]byte(txtContent)); err != nil {
		return nil, "", fmt.Errorf("write txt: %w", err)
	}

	// 3. raw_request.txt (最后一次LLM调用的原始请求)
	rawRequest := getLastRawRequest(db, payload.SourceSessionID)
	if rawRequest != "" {
		w, err = zw.Create(fmt.Sprintf("%s-raw_request.txt", safeTitle))
		if err != nil {
			return nil, "", fmt.Errorf("create raw_request entry: %w", err)
		}
		if _, err := w.Write([]byte(rawRequest)); err != nil {
			return nil, "", fmt.Errorf("write raw_request: %w", err)
		}
	}

	if err := zw.Close(); err != nil {
		return nil, "", fmt.Errorf("close zip: %w", err)
	}

	return buf, filename, nil
}

func getLastRawRequest(db *gorm.DB, sessionID string) string {
	if db == nil || sessionID == "" {
		return ""
	}

	// 查询 session 关联的 task
	task, err := database.GetTaskBySessionID(db, sessionID)
	if err != nil || task == nil {
		return ""
	}

	// 查询该 task 最后一次 LLM API 调用的日志
	var log models.CommandLog
	err = db.Model(&models.CommandLog{}).
		Where("source = ? AND source_id = ?", "llm_api_call", task.ID).
		Order("created_at DESC").
		First(&log).Error
	if err != nil {
		return ""
	}

	// 从 fields 中查找 raw_request
	for _, field := range log.Fields {
		if field.Key == "raw_request" && field.Value != "" {
			return field.Value
		}
	}

	// 备选：从 stdinData 中读取
	if log.StdinData != "" {
		return log.StdinData
	}

	return ""
}

func sanitizeExportFilename(input string) string {
	return strings.TrimSpace(
		strings.ToLower(
			strings.NewReplacer(
				" ", "-",
				"_", "-",
				"/", "-",
				"\\", "-",
				":", "-",
				"*", "",
				"?", "",
				"\"", "",
				"<", "",
				">", "",
				"|", "",
			).Replace(input),
		),
	)
}

func escapeXml(text string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	).Replace(text)
}

func serializeMemoriesToXML(payload SessionTransferPayload) string {
	var b strings.Builder
	sessionTitle := ""
	if payload.Session != nil {
		sessionTitle = payload.Session.Title
	}
	if sessionTitle == "" {
		sessionTitle = "未命名会话"
	}

	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<chat-export>\n")
	b.WriteString(fmt.Sprintf("  <session-title>%s</session-title>\n", escapeXml(sessionTitle)))
	b.WriteString(fmt.Sprintf("  <exported-at>%s</exported-at>\n", time.Now().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("  <message-count>%d</message-count>\n", len(payload.Messages)))
	b.WriteString(fmt.Sprintf("  <memory-count>%d</memory-count>\n", len(payload.MemoryEntries)))
	b.WriteString("  <memories>\n")

	for _, entry := range payload.MemoryEntries {
		role := entry.Role
		if role == "" {
			role = "assistant"
		}
		entryTime := ""
		if entry.Created > 0 {
			entryTime = time.UnixMilli(entry.Created).Format(time.RFC3339)
		}
		b.WriteString(fmt.Sprintf("    <memory role=\"%s\" kind=\"%s\" time=\"%s\">\n", role, escapeXml(entry.EntryKind), entryTime))

		if entry.Content != "" {
			b.WriteString(fmt.Sprintf("      <content>%s</content>\n", escapeXml(entry.Content)))
		}

		if entry.RawOutput != "" {
			b.WriteString(fmt.Sprintf("      <raw-output>%s</raw-output>\n", escapeXml(entry.RawOutput)))
		}

		if entry.ToolName != "" {
			b.WriteString(fmt.Sprintf("      <tool name=\"%s\" status=\"%s\">\n", escapeXml(entry.ToolName), escapeXml(entry.ToolStatus)))

			if entry.ToolInputJSON != "" {
				b.WriteString(fmt.Sprintf("        <input>%s</input>\n", escapeXml(entry.ToolInputJSON)))
			}

			if entry.ToolOutput != "" {
				b.WriteString(fmt.Sprintf("        <output>%s</output>\n", escapeXml(entry.ToolOutput)))
			}

			if entry.ToolError != "" {
				b.WriteString(fmt.Sprintf("        <error>%s</error>\n", escapeXml(entry.ToolError)))
			}

			b.WriteString("      </tool>\n")
		}

		b.WriteString("    </memory>\n")
	}

	b.WriteString("  </memories>\n")
	b.WriteString("</chat-export>\n")

	return b.String()
}

func (h *SessionHandler) ImportSessionTransfer(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
		return
	}

	currentSession, err := storage.GetSession(h.db, sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "会话不存在"})
		return
	}

	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败"})
		return
	}

	var jsonText string

	// 判断是否为 ZIP
	if len(body) > 4 && body[0] == 0x50 && body[1] == 0x4B && body[2] == 0x03 && body[3] == 0x04 {
		// ZIP 文件
		jsonText, err = extractJsonFromZipBytes(body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("解析 ZIP 失败: %v", err)})
			return
		}
	} else {
		// 纯 JSON
		jsonText = string(body)
	}

	var req SessionTransferPayload
	if err := json.Unmarshal([]byte(jsonText), &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "导入 JSON 格式错误"})
		return
	}

	if req.Kind != "" && req.Kind != sessionTransferKind {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不是受支持的会话导入文件"})
		return
	}
	if req.Version != 0 && req.Version != sessionTransferVersion {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("暂不支持导入版本 %d", req.Version)})
		return
	}

	normalizedMessages, messageIDMap, partIDMap, err := normalizeImportedMessages(sessionID, req.Messages)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	normalizedMemoryEntries, err := normalizeImportedMemoryEntries(sessionID, req.MemoryEntries, messageIDMap, partIDMap)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := storage.ReplaceSessionMessagesWithParts(h.db, sessionID, normalizedMessages); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "覆盖聊天记录失败"})
		return
	}
	if err := storage.ReplaceSessionMemoryWithEntries(h.db, sessionID, normalizedMemoryEntries); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "覆盖历史记忆失败"})
		return
	}

	importedTitle := currentSession.Title
	importedTokens := currentSession.Tokens
	importedSummary := currentSession.Summary
	if req.Session != nil {
		if title := strings.TrimSpace(req.Session.Title); title != "" {
			importedTitle = title
		}
		if req.Session.Tokens != nil {
			importedTokens = req.Session.Tokens
		}
		if req.Session.Summary != nil {
			importedSummary = req.Session.Summary
		}
	}

	if _, err := storage.UpdateSessionByCallback(h.db, sessionID, func(draft *types.Info) error {
		draft.Title = importedTitle
		draft.Tokens = importedTokens
		draft.Summary = importedSummary
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新会话信息失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":             "导入成功，已覆盖当前会话的聊天记录和记忆",
		"sessionId":           sessionID,
		"importedMessages":    len(normalizedMessages),
		"importedMemories":    len(normalizedMemoryEntries),
		"sourceSessionId":     req.SourceSessionID,
		"appliedSessionTitle": importedTitle,
	})
}

func extractJsonFromZipBytes(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}

	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".json") && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			content, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}
			return string(content), nil
		}
	}

	return "", fmt.Errorf("ZIP 文件中找不到 JSON 文件")
}

func normalizeImportedMessages(targetSessionID string, input []*types.WithParts) ([]*types.WithParts, map[string]string, map[string]string, error) {
	messageIDMap := make(map[string]string, len(input))
	partIDMap := map[string]string{}
	output := make([]*types.WithParts, 0, len(input))

	for index, message := range input {
		if message == nil || message.Info == nil {
			continue
		}

		cloned, err := cloneMessageWithParts(message)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("clone imported message #%d: %w", index+1, err)
		}

		oldMessageID := strings.TrimSpace(cloned.Info.ID)
		if oldMessageID == "" {
			return nil, nil, nil, fmt.Errorf("message #%d 缺少 id", index+1)
		}

		newMessageID := util.Ascending("message")
		messageIDMap[oldMessageID] = newMessageID
		cloned.Info.ID = newMessageID
		cloned.Info.SessionID = targetSessionID
		if cloned.Info.Time.Created == 0 {
			cloned.Info.Time.Created = time.Now().UnixMilli() + int64(index)
		}

		for partIndex, part := range cloned.Parts {
			if part == nil {
				continue
			}
			oldPartID := strings.TrimSpace(part.ID)
			if oldPartID == "" {
				return nil, nil, nil, fmt.Errorf("message #%d 的 part #%d 缺少 id", index+1, partIndex+1)
			}
			newPartID := util.Ascending("part")
			partIDMap[oldPartID] = newPartID
			part.ID = newPartID
			part.SessionID = targetSessionID
			part.MessageID = newMessageID
			if part.Time == nil {
				part.Time = &types.PartTime{Created: cloned.Info.Time.Created}
			} else if part.Time.Created == 0 {
				part.Time.Created = cloned.Info.Time.Created
			}
		}

		output = append(output, cloned)
	}

	return output, messageIDMap, partIDMap, nil
}

func normalizeImportedMemoryEntries(targetSessionID string, input []*types.MemoryEntry, messageIDMap, partIDMap map[string]string) ([]*types.MemoryEntry, error) {
	output := make([]*types.MemoryEntry, 0, len(input))

	for index, entry := range input {
		if entry == nil {
			continue
		}
		cloned, err := cloneMemoryEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("clone imported memory #%d: %w", index+1, err)
		}

		cloned.ID = 0
		cloned.SessionID = targetSessionID
		if mapped := messageIDMap[cloned.SourceMessageID]; mapped != "" {
			cloned.SourceMessageID = mapped
		}
		if mapped := partIDMap[cloned.SourcePartID]; mapped != "" {
			cloned.SourcePartID = mapped
		}
		if cloned.Created == 0 {
			cloned.Created = time.Now().UnixMilli() + int64(index)
		}
		if cloned.Updated == 0 {
			cloned.Updated = cloned.Created
		}

		output = append(output, cloned)
	}

	return output, nil
}

func cloneMessageWithParts(input *types.WithParts) (*types.WithParts, error) {
	if input == nil {
		return nil, nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var cloned types.WithParts
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}

func cloneMemoryEntry(input *types.MemoryEntry) (*types.MemoryEntry, error) {
	if input == nil {
		return nil, nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var cloned types.MemoryEntry
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, err
	}
	return &cloned, nil
}
