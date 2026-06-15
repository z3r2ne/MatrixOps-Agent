package tool

import (
	"errors"
	"fmt"
	"strings"

	"pkgs/db/models"
	"pkgs/embedding"
	"pkgs/memorysearch"

	"gorm.io/gorm"
)

type MemorySearchTool struct {
	db        *gorm.DB
	sessionID string
}

func NewMemorySearchTool(db *gorm.DB, sessionID string) MemorySearchTool {
	return MemorySearchTool{db: db, sessionID: sessionID}
}

func (MemorySearchTool) Name() string { return memorysearch.MemorySearchToolName }

func (MemorySearchTool) VerbosName() string { return "记忆检索" }

func (MemorySearchTool) Description() string {
	return "检索被压缩前的历史记忆与项目记忆库。可传入多个语义搜索语句和关键词，返回最相关的记忆片段。"
}

func (MemorySearchTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"queries": map[string]interface{}{
			"type":        "array",
			"description": "语义搜索语句，可传多个",
			"items": map[string]interface{}{
				"type": "string",
			},
		},
		"keywords": map[string]interface{}{
			"type":        "array",
			"description": "关键词搜索，可传多个",
			"items": map[string]interface{}{
				"type": "string",
			},
		},
		"top_k": map[string]interface{}{
			"type":        "number",
			"description": "返回结果数量，默认 8，最大 20",
		},
	}, nil)
}

func (t MemorySearchTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	if err := CheckContext(ctx); err != nil {
		return Result{IsError: true}, err
	}
	if t.db == nil {
		return Result{IsError: true}, errors.New("memory_search: database unavailable")
	}
	if !embedding.HasEnabledEmbeddingConfig(t.db) {
		return Result{IsError: true}, errors.New("memory_search: embedding is not configured")
	}

	sessionID := strings.TrimSpace(ctx.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(t.sessionID)
	}
	if sessionID == "" {
		return Result{IsError: true}, errors.New("memory_search: session id is required")
	}

	queries := parseStringSlice(input["queries"])
	keywords := parseStringSlice(input["keywords"])
	if len(queries) == 0 && len(keywords) == 0 {
		return Result{IsError: true}, errors.New("memory_search: at least one query or keyword is required")
	}

	topK := models.DefaultMemorySearchTopK
	if rawTopK, ok := input["top_k"].(float64); ok && rawTopK > 0 {
		topK = int(rawTopK)
	}
	if topK > 20 {
		topK = 20
	}

	libraryIDs, err := memorysearch.ResolveLibraryIDsForSession(t.db, sessionID)
	if err != nil {
		return Result{IsError: true}, fmt.Errorf("memory_search: resolve libraries: %w", err)
	}

	results, err := memorysearch.SearchMulti(ctx.Context, t.db, sessionID, libraryIDs, queries, keywords, topK)
	if err != nil {
		return Result{IsError: true}, fmt.Errorf("memory_search: %w", err)
	}
	return Result{Content: memorysearch.FormatSearchResults(results)}, nil
}

func IsMemorySearchTool(name string) bool {
	return name == memorysearch.MemorySearchToolName
}

func RegisterMemorySearchTools(registry *Registry, db *gorm.DB, sessionID string) {
	if registry == nil || db == nil || !embedding.HasEnabledEmbeddingConfig(db) {
		return
	}
	registry.Register(NewMemorySearchTool(db, sessionID))
}

func parseStringSlice(raw interface{}) []string {
	switch values := raw.(type) {
	case []string:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if text, ok := value.(string); ok {
				if trimmed := strings.TrimSpace(text); trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
		return out
	default:
		return nil
	}
}
