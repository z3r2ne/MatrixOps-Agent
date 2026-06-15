package session

import (
	"testing"

	"matrixops-agent/types"
	"pkgs/db/storage"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openEmitterOrderTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := storage.InitStorage(db); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	return db
}

func TestEmitterPersistsAssistantTextPartBeforeToolPart(t *testing.T) {
	db := openEmitterOrderTestDB(t)
	sessionID := "session-order-test"
	messageID := "message-order-test"

	emitter := NewEmitter(db, sessionID)

	message := &types.MessageInfo{
		ID:        messageID,
		SessionID: sessionID,
		Role:      types.RoleAssistant,
		Time:      types.MessageTime{Created: 100},
	}
	if _, err := emitter.UpdateMessage(message); err != nil {
		t.Fatalf("UpdateMessage: %v", err)
	}

	textPart := &types.Part{
		ID:        "part-text",
		MessageID: messageID,
		SessionID: sessionID,
		Type:      types.PartTypeText,
		Text:      "我先快速梳理了仓库结构。",
		Time:      &types.PartTime{Start: 101, Created: 101, End: 102},
	}
	if _, err := emitter.UpdatePart(textPart); err != nil {
		t.Fatalf("UpdatePart(text): %v", err)
	}

	toolPart := &types.Part{
		ID:        "part-tool",
		MessageID: messageID,
		SessionID: sessionID,
		Type:      types.PartTypeTool,
		Time:      &types.PartTime{Start: 103, Created: 103},
		Tool: &types.ToolPart{
			Name:   "read",
			CallID: "call-1",
			State: types.ToolState{
				Status: "running",
				Input:  map[string]interface{}{"path": "README.md"},
				Time:   types.PartTime{Start: 103, Created: 103},
			},
		},
	}
	if _, err := emitter.UpdatePart(toolPart); err != nil {
		t.Fatalf("UpdatePart(tool): %v", err)
	}

	withParts, err := storage.GetMessageWithPartsLight(db, messageID)
	if err != nil {
		t.Fatalf("GetMessageWithPartsLight: %v", err)
	}
	if withParts == nil {
		t.Fatal("expected message with parts")
	}
	if len(withParts.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(withParts.Parts))
	}
	if withParts.Parts[0].Type != types.PartTypeText {
		t.Fatalf("expected first part to be text, got %q", withParts.Parts[0].Type)
	}
	if withParts.Parts[1].Type != types.PartTypeTool {
		t.Fatalf("expected second part to be tool, got %q", withParts.Parts[1].Type)
	}
	if withParts.Parts[0].Text != "我先快速梳理了仓库结构。" {
		t.Fatalf("unexpected first part text: %q", withParts.Parts[0].Text)
	}
	if withParts.Parts[1].Tool == nil || withParts.Parts[1].Tool.Name != "read" {
		t.Fatalf("unexpected tool part: %#v", withParts.Parts[1].Tool)
	}
}
