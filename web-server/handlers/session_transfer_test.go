package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"matrixops-agent/types"
	"pkgs/db/models"
	"pkgs/db/storage"

	"github.com/gin-gonic/gin"
)

func TestExportSessionTransfer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSessionHandlerTestDB(t)
	sessionID := "session-export"

	if err := db.Create(&models.Session{
		ID:        sessionID,
		ProjectID: "project-1",
		Title:     "export session",
		Version:   "1",
		Created:   1,
		Updated:   1,
	}).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := storage.ReplaceSessionMessagesWithParts(db, sessionID, []*types.WithParts{
		{
			Info: &types.MessageInfo{
				ID:        "message-export-1",
				SessionID: sessionID,
				Role:      types.RoleUser,
				Time:      types.MessageTime{Created: 10},
			},
			Parts: []*types.Part{
				{
					ID:        "part-export-1",
					MessageID: "message-export-1",
					SessionID: sessionID,
					Type:      types.PartTypeText,
					Text:      "你好，导出一下",
				},
			},
		},
	}); err != nil {
		t.Fatalf("replace session messages: %v", err)
	}

	if err := storage.ReplaceSessionMemoryWithEntries(db, sessionID, []*types.MemoryEntry{
		{
			SessionID:  sessionID,
			EntryKind:  "history",
			Role:       "user",
			Content:    "导出记忆",
			Sequence:   1,
			TokenCount: 3,
			Created:    11,
			Updated:    11,
		},
	}); err != nil {
		t.Fatalf("replace session memory: %v", err)
	}

	router := gin.New()
	handler := NewSessionHandler(db)
	router.GET("/sessions/:id/export", handler.ExportSessionTransfer)

	req := httptest.NewRequest(http.MethodGet, "/sessions/"+sessionID+"/export", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	// Handler returns ZIP; extract the data.json entry
	zr, err := zip.NewReader(bytes.NewReader(resp.Body.Bytes()), int64(resp.Body.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	var body SessionTransferPayload
	found := false
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "-data.json") {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open zip entry: %v", err)
			}
			jsonData, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatalf("read zip entry: %v", err)
			}
			if err := json.Unmarshal(jsonData, &body); err != nil {
				t.Fatalf("unmarshal json: %v", err)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("zip missing *-data.json entry")
	}

	if body.Kind != sessionTransferKind {
		t.Fatalf("kind = %q, want %q", body.Kind, sessionTransferKind)
	}
	if body.Version != sessionTransferVersion {
		t.Fatalf("version = %d, want %d", body.Version, sessionTransferVersion)
	}
	if body.Session == nil || body.Session.ID != sessionID {
		t.Fatalf("unexpected session: %+v", body.Session)
	}
	if len(body.Messages) != 1 || len(body.Messages[0].Parts) != 1 || body.Messages[0].Parts[0].Text != "你好，导出一下" {
		t.Fatalf("unexpected messages: %+v", body.Messages)
	}
	if len(body.MemoryEntries) != 1 || body.MemoryEntries[0].Content != "导出记忆" {
		t.Fatalf("unexpected memory entries: %+v", body.MemoryEntries)
	}
}

func TestImportSessionTransferOverwritesExistingData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupSessionHandlerTestDB(t)
	sessionID := "session-import"

	if err := db.Create(&models.Session{
		ID:        sessionID,
		ProjectID: "project-1",
		Title:     "old title",
		Version:   "1",
		Created:   1,
		Updated:   1,
	}).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := storage.ReplaceSessionMessagesWithParts(db, sessionID, []*types.WithParts{
		{
			Info: &types.MessageInfo{
				ID:        "message-old-1",
				SessionID: sessionID,
				Role:      types.RoleAssistant,
				Time:      types.MessageTime{Created: 1},
			},
			Parts: []*types.Part{
				{
					ID:        "part-old-1",
					MessageID: "message-old-1",
					SessionID: sessionID,
					Type:      types.PartTypeText,
					Text:      "旧消息",
				},
			},
		},
	}); err != nil {
		t.Fatalf("replace session messages: %v", err)
	}

	if err := storage.ReplaceSessionMemoryWithEntries(db, sessionID, []*types.MemoryEntry{
		{
			SessionID:  sessionID,
			EntryKind:  "history",
			Role:       "assistant",
			Content:    "旧记忆",
			Sequence:   1,
			TokenCount: 2,
			Created:    1,
			Updated:    1,
		},
	}); err != nil {
		t.Fatalf("replace session memory: %v", err)
	}

	payload := SessionTransferPayload{
		Kind:            sessionTransferKind,
		Version:         sessionTransferVersion,
		SourceSessionID: "another-session",
		Session: &types.Info{
			Title: "导入后的标题",
			Tokens: &types.MessageTokens{
				Input: 42,
			},
		},
		Messages: []*types.WithParts{
			{
				Info: &types.MessageInfo{
					ID:        "import-message-1",
					SessionID: "another-session",
					Role:      types.RoleUser,
					Time:      types.MessageTime{Created: 20},
				},
				Parts: []*types.Part{
					{
						ID:        "import-part-1",
						MessageID: "import-message-1",
						SessionID: "another-session",
						Type:      types.PartTypeText,
						Text:      "新的用户消息",
					},
				},
			},
			{
				Info: &types.MessageInfo{
					ID:        "import-message-2",
					SessionID: "another-session",
					Role:      types.RoleAssistant,
					Time:      types.MessageTime{Created: 21},
				},
				Parts: []*types.Part{
					{
						ID:        "import-part-2",
						MessageID: "import-message-2",
						SessionID: "another-session",
						Type:      types.PartTypeText,
						Text:      "新的助手消息",
					},
				},
			},
		},
		MemoryEntries: []*types.MemoryEntry{
			{
				ID:              999,
				SessionID:       "another-session",
				SourceMessageID: "import-message-2",
				SourcePartID:    "import-part-2",
				EntryKind:       "history",
				Role:            "assistant",
				Content:         "新的记忆",
				Sequence:        5,
				TokenCount:      4,
				Created:         30,
				Updated:         30,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	router := gin.New()
	handler := NewSessionHandler(db)
	router.POST("/sessions/:id/import", handler.ImportSessionTransfer)

	req := httptest.NewRequest(http.MethodPost, "/sessions/"+sessionID+"/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	session, err := storage.GetSession(db, sessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.Title != "导入后的标题" {
		t.Fatalf("title = %q, want %q", session.Title, "导入后的标题")
	}
	if session.Tokens == nil || session.Tokens.Input != 42 {
		t.Fatalf("unexpected session tokens: %+v", session.Tokens)
	}

	messages, err := storage.GetMessageWithPartsBySessionID(db, sessionID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}
	if messages[0].Parts[0].Text != "新的用户消息" || messages[1].Parts[0].Text != "新的助手消息" {
		t.Fatalf("unexpected imported messages: %+v", messages)
	}
	if messages[0].Info.ID == "import-message-1" || messages[1].Info.ID == "import-message-2" {
		t.Fatalf("expected message ids to be remapped, got %+v", []string{messages[0].Info.ID, messages[1].Info.ID})
	}

	memoryEntries, err := storage.ListMemoryEntriesBySession(db, sessionID)
	if err != nil {
		t.Fatalf("get memory entries: %v", err)
	}
	if len(memoryEntries) != 1 {
		t.Fatalf("memory entry count = %d, want 1", len(memoryEntries))
	}
	if memoryEntries[0].Content != "新的记忆" {
		t.Fatalf("unexpected memory content: %+v", memoryEntries[0])
	}
	if memoryEntries[0].ID == 999 {
		t.Fatalf("expected memory id to be regenerated, got %d", memoryEntries[0].ID)
	}
	if memoryEntries[0].SourceMessageID == "import-message-2" || memoryEntries[0].SourcePartID == "import-part-2" {
		t.Fatalf("expected memory source ids to be remapped, got %+v", memoryEntries[0])
	}
	if memoryEntries[0].SourceMessageID != messages[1].Info.ID {
		t.Fatalf("memory source message id = %q, want %q", memoryEntries[0].SourceMessageID, messages[1].Info.ID)
	}
	if memoryEntries[0].SourcePartID != messages[1].Parts[0].ID {
		t.Fatalf("memory source part id = %q, want %q", memoryEntries[0].SourcePartID, messages[1].Parts[0].ID)
	}
}
