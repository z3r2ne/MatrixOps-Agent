package task_runner

import (
	"os"
	"path/filepath"
	"testing"

	agentsession "matrixops-agent/session"
	agenttool "matrixops-agent/tool"
	"pkgs/db/models"
	"pkgs/testutil"
)

func TestWrapDeliverUserMessage_ForwardsAttachmentAfterMessageTool(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	if err := db.AutoMigrate(&models.Message{}, &models.Part{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "demo.png")
	if err := os.WriteFile(imgPath, []byte{0x89, 0x50, 0x4e, 0x47}, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	hub := &taskMessageCaptureHub{}
	runtime := &TaskRuntime{
		taskID:                           9,
		db:                               db,
		emitter:                          NewEmitter(hub, db, 9),
		workDir:                          dir,
		forwardedAssistantKeys:           make(map[string]struct{}),
		forwardedAssistantAttachmentKeys: make(map[string]struct{}),
	}
	runtime.turnForwardToWeChat.Store(true)

	sessionID := "session-wechat-forward"
	runtime.sessionEmitter = agentsession.NewEmitter(db, sessionID)

	deliver := runtime.wrapDeliverUserMessage()
	if err := deliver(agenttool.Context{Directory: dir}, agenttool.UserDeliveryParams{
		Caption:  "重新发送图片",
		FilePath: imgPath,
	}); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	var hasAttachment bool
	var hasText bool
	for _, msg := range hub.taskMessages {
		switch msg.Type {
		case TaskMessageTypeAssistantAttachment:
			hasAttachment = true
		case "message":
			hasText = true
		}
	}
	if !hasText {
		t.Fatal("expected text task message from message tool delivery")
	}
	if !hasAttachment {
		t.Fatalf("expected assistant_attachment, got %#v", hub.taskMessages)
	}
}

func TestWrapDeliverUserMessage_SkipsWhenNotWeChatTurn(t *testing.T) {
	db := testutil.OpenTaskTestDB(t)
	if err := db.AutoMigrate(&models.Message{}, &models.Part{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "demo.png")
	if err := os.WriteFile(imgPath, []byte("png"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	hub := &taskMessageCaptureHub{}
	runtime := &TaskRuntime{
		taskID:  10,
		db:      db,
		emitter: NewEmitter(hub, db, 10),
		workDir: dir,
	}
	runtime.sessionEmitter = agentsession.NewEmitter(db, "session-frontend")

	deliver := runtime.wrapDeliverUserMessage()
	if err := deliver(agenttool.Context{Directory: dir}, agenttool.UserDeliveryParams{
		FilePath: imgPath,
	}); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if len(hub.taskMessages) != 0 {
		t.Fatalf("frontend turn should not forward to wechat, got %d messages", len(hub.taskMessages))
	}
}
