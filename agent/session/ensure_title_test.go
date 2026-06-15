package session

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"matrixops-agent/types"
	coreagent "matrixops.local/core_agent"
	"pkgs/db/models"
	"pkgs/db/storage"
)

const titleTestChatCompletionSSE = "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-test\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"修复 agent 标题生成\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2,\"prompt_tokens_details\":{\"cached_tokens\":0},\"completion_tokens_details\":{\"reasoning_tokens\":0}}}\n\ndata: [DONE]\n\n"

func newTitleNativeChatTestServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.NotFound(w, r)
			return
		}
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, titleTestChatCompletionSSE)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestEnsureTitleGeneratesPersistsAndEmits(t *testing.T) {
	db := openEmitterOrderTestDB(t)
	sessionID := "session-title-test"
	messageID := "message-title-test"

	sessionInfo := &Info{
		ID:        sessionID,
		ProjectID: "project-1",
		Directory: t.TempDir(),
		Time: TimeInfo{
			Created: 1,
			Updated: 1,
		},
	}
	if err := storage.UpdateSession(db, sessionInfo); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	emitter := NewEmitter(db, sessionID)
	message := &MessageInfo{
		ID:        messageID,
		SessionID: sessionID,
		Role:      RoleUser,
		Time:      MessageTime{Created: 1},
	}
	if _, err := emitter.UpdateMessage(message); err != nil {
		t.Fatalf("UpdateMessage: %v", err)
	}
	if _, err := emitter.UpdatePart(&Part{
		ID:        "part-title-test",
		MessageID: messageID,
		SessionID: sessionID,
		Type:      types.PartTypeText,
		Text:      "修复 agent 标题生成不生效",
		Time:      &PartTime{Created: 1},
	}); err != nil {
		t.Fatalf("UpdatePart: %v", err)
	}

	srv, callCount := newTitleNativeChatTestServer(t)
	runner := &AgentRunner{session: sessionInfo, db: db, emitter: emitter}

	emittedTitle := ""
	emitter.On(EventSessionTitleUpdated, func(args ...interface{}) {
		title, _ := args[0].(string)
		emittedTitle = title
	})

	err := runner.ensureTitle(&RuntimeConfig{
		Ctx: context.Background(),
		LLMConfig: &models.LLMConfig{
			Name:                  "test-config",
			BaseURL:               strings.TrimSuffix(srv.URL, "/") + "/v1",
			APIKey:                "test-key",
			APIType:               models.LLMAPITypeChat,
			NativeOpenAIToolCalls: true,
		},
		Model: "gpt-test",
	})
	if err != nil {
		t.Fatalf("ensureTitle: %v", err)
	}

	if *callCount != 1 {
		t.Fatalf("expected 1 native chat completion request, got %d", *callCount)
	}

	stored, err := storage.GetSession(db, sessionID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if stored.Title != "修复 agent 标题生成" {
		t.Fatalf("unexpected stored title: %q", stored.Title)
	}
	if sessionInfo.Title != "修复 agent 标题生成" {
		t.Fatalf("unexpected in-memory title: %q", sessionInfo.Title)
	}
	if emittedTitle != "修复 agent 标题生成" {
		t.Fatalf("unexpected emitted title: %q", emittedTitle)
	}
}

func TestEnsureTitleSkipsWhenMultipleRealUserMessagesExist(t *testing.T) {
	db := openEmitterOrderTestDB(t)
	sessionID := "session-title-skip-test"
	sessionInfo := &Info{
		ID:        sessionID,
		ProjectID: "project-1",
		Directory: t.TempDir(),
		Time: TimeInfo{
			Created: 1,
			Updated: 1,
		},
	}
	if err := storage.UpdateSession(db, sessionInfo); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	emitter := NewEmitter(db, sessionID)
	for index, text := range []string{"第一次输入", "第二次输入"} {
		messageID := "message-skip-test-" + string(rune('a'+index))
		if _, err := emitter.UpdateMessage(&MessageInfo{
			ID:        messageID,
			SessionID: sessionID,
			Role:      RoleUser,
			Time:      MessageTime{Created: int64(index + 1)},
		}); err != nil {
			t.Fatalf("UpdateMessage(%d): %v", index, err)
		}
		if _, err := emitter.UpdatePart(&Part{
			ID:        "part-skip-test-" + string(rune('a'+index)),
			MessageID: messageID,
			SessionID: sessionID,
			Type:      types.PartTypeText,
			Text:      text,
			Time:      &PartTime{Created: int64(index + 1)},
		}); err != nil {
			t.Fatalf("UpdatePart(%d): %v", index, err)
		}
	}

	srv, callCount := newTitleNativeChatTestServer(t)
	runner := &AgentRunner{session: sessionInfo, db: db, emitter: emitter}

	if err := runner.ensureTitle(&RuntimeConfig{
		Ctx: context.Background(),
		LLMConfig: &models.LLMConfig{
			Name:                  "test-config",
			BaseURL:               strings.TrimSuffix(srv.URL, "/") + "/v1",
			APIKey:                "test-key",
			APIType:               models.LLMAPITypeChat,
			NativeOpenAIToolCalls: true,
		},
		Model: "gpt-test",
	}); err != nil {
		t.Fatalf("ensureTitle: %v", err)
	}

	if *callCount != 0 {
		t.Fatalf("expected no llm request, got %d", *callCount)
	}
	stored, err := storage.GetSession(db, sessionID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if stored.Title != "" {
		t.Fatalf("expected session title to remain empty, got %q", stored.Title)
	}
}

func TestCollectStreamV2AnswerTextReadsRawTextWhenFinishTurnUnset(t *testing.T) {
	out := &coreagent.StreamOutput{
		RawTextReader: strings.NewReader("compaction summary"),
		Wait:          func() error { return nil },
	}

	text, err := collectStreamV2AnswerText(out)
	if err != nil {
		t.Fatalf("collectStreamV2AnswerText: %v", err)
	}
	if text != "compaction summary" {
		t.Fatalf("text = %q, want %q", text, "compaction summary")
	}
}

func TestCollectStreamV2AnswerTextPrefersRawTextReader(t *testing.T) {
	out := &coreagent.StreamOutput{
		RawTextReader:                   strings.NewReader("raw title"),
		ContentReader:                   strings.NewReader("content title"),
		NativeAssistantTextFinishesTurn: true,
		Wait:                            func() error { return nil },
	}

	title, err := collectStreamV2AnswerText(out)
	if err != nil {
		t.Fatalf("collectStreamV2AnswerText: %v", err)
	}
	if title != "raw title" {
		t.Fatalf("title = %q, want %q", title, "raw title")
	}
}
