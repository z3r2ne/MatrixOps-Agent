package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	agentllm "matrixops-agent/llm"
	agentprovider "matrixops-agent/provider"
	builtinworkers "matrixops.local/core_agent/workersv2/builtin"
	builtinskills "matrixops-agent/skills/builtin"
	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExecuteCLIChatTurnReusesSessionHistory(t *testing.T) {
	db := openCLIChatTestDB(t)
	workDir := t.TempDir()
	project := createCLIChatTestProject(t, db, workDir)

	mock := agentprovider.NewMockClientWithStreamCallback(func(request agentllm.ChatRequest) (<-chan agentllm.StreamEvent, error) {
		index := len(mockRequests) + 1
		mockRequests = append(mockRequests, request)
		events := make(chan agentllm.StreamEvent, 4)
		go func() {
			defer close(events)
			events <- agentllm.StreamEvent{
				Type: "text-delta",
				Text: fmt.Sprintf(`{"call_tool":"answer","params":"reply-%d"}`, index),
			}
			events <- agentllm.StreamEvent{
				Type:   "finish",
				Finish: "stop",
				Usage:  &agentllm.Usage{InputTokens: 8, OutputTokens: 4},
			}
		}()
		return events, nil
	})

	sessionID, err := executeCLIChatTurn(context.Background(), db, newCLIChatHub(io.Discard, io.Discard, strings.NewReader(""), false), cliChatRunOptions{
		ProjectID: strconv.FormatUint(uint64(project.ID), 10),
		WorkDir:   workDir,
		Prompt:    "first question",
		LLMClient: mock,
	})
	if err != nil {
		t.Fatalf("first executeCLIChatTurn: %v", err)
	}
	if strings.TrimSpace(sessionID) == "" {
		t.Fatal("expected session id to be created")
	}

	sessionID2, err := executeCLIChatTurn(context.Background(), db, newCLIChatHub(io.Discard, io.Discard, strings.NewReader(""), false), cliChatRunOptions{
		SessionID: sessionID,
		Prompt:    "second question",
		LLMClient: mock,
	})
	if err != nil {
		t.Fatalf("second executeCLIChatTurn: %v", err)
	}
	if sessionID2 != sessionID {
		t.Fatalf("session id mismatch: got %q want %q", sessionID2, sessionID)
	}

	if len(mockRequests) != 2 {
		t.Fatalf("expected 2 LLM requests, got %d", len(mockRequests))
	}

	secondRequestDump := flattenRequestMessages(mockRequests[1].Messages)
	if !strings.Contains(secondRequestDump, "first question") {
		t.Fatalf("expected second request to include previous user input, got: %s", secondRequestDump)
	}
	if !strings.Contains(secondRequestDump, "reply-1") {
		t.Fatalf("expected second request to include previous assistant output, got: %s", secondRequestDump)
	}
}

func TestResolveProjectForCLIPrefersLongestPathMatch(t *testing.T) {
	db := openCLIChatTestDB(t)
	root := t.TempDir()
	parentPath := filepath.Join(root, "workspace")
	childPath := filepath.Join(parentPath, "nested")

	parent := createCLIChatTestProject(t, db, parentPath)
	child := createCLIChatTestProject(t, db, childPath)

	target := filepath.Join(childPath, "pkg", "feature")
	project, err := resolveProjectForCLI(db, "", target)
	if err != nil {
		t.Fatalf("resolveProjectForCLI: %v", err)
	}
	if project.ID != child.ID {
		t.Fatalf("expected child project %d, got %d (parent=%d)", child.ID, project.ID, parent.ID)
	}
}

var mockRequests []agentllm.ChatRequest

func openCLIChatTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "cli-chat.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.InitDB(db, builtinworkers.ReadAll(), builtinskills.ReadAll()); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if err := db.Model(&models.ModelSettings{}).
		Where("name = ?", database.DefaultModelSettingsName).
		Update("native_open_ai_tool_calls", false).Error; err != nil {
		t.Fatalf("disable native openai tool calls: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	mockRequests = nil
	return db
}

func createCLIChatTestProject(t *testing.T, db *gorm.DB, path string) *models.Project {
	t.Helper()

	project := &models.Project{
		Name:         filepath.Base(path),
		Path:         path,
		WorktreePath: path,
		YoloMode:     true,
	}
	if err := db.Create(project).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	return project
}

func flattenRequestMessages(messages []*agentllm.ModelMessage) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%v", message.Role, message.Content))
	}
	return strings.Join(parts, "\n")
}
