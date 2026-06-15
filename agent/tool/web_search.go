package tool

import (
	"errors"
	"fmt"

	database "pkgs/db"
	"pkgs/search"

	"gorm.io/gorm"
)

const WebSearchToolName = "web_search"

type WebSearchTool struct {
	db *gorm.DB
}

func NewWebSearchTool(db *gorm.DB) WebSearchTool {
	return WebSearchTool{db: db}
}

func (WebSearchTool) Name() string { return WebSearchToolName }

func (WebSearchTool) VerbosName() string { return "网页搜索" }

func (WebSearchTool) Description() string {
	return "搜索互联网内容，获取实时信息和网页摘要。适用于需要最新资料、文档或新闻的场景。"
}

func (WebSearchTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "搜索关键词，尽量具体明确",
		},
		"limit": map[string]interface{}{
			"type":        "number",
			"description": "返回结果数量，默认 5，最大 20",
		},
		"enable_page_crawling": map[string]interface{}{
			"type":        "boolean",
			"description": "是否抓取完整页面内容，默认 false",
		},
	}, []string{"query"})
}

func (t WebSearchTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	if err := CheckContext(ctx); err != nil {
		return Result{IsError: true}, err
	}
	if t.db == nil {
		return Result{IsError: true}, errors.New("web_search: database unavailable")
	}

	config, err := database.GetActiveSearchConfig(t.db)
	if err != nil || config == nil {
		return Result{IsError: true}, errors.New("web_search: no enabled search plugin configured")
	}

	query, _ := input["query"].(string)
	limit := 5
	if rawLimit, ok := input["limit"].(float64); ok && rawLimit > 0 {
		limit = int(rawLimit)
	}
	enablePageCrawling, _ := input["enable_page_crawling"].(bool)

	response, err := search.Search(*config, query, limit, enablePageCrawling)
	if err != nil {
		return Result{IsError: true}, fmt.Errorf("web_search: %w", err)
	}
	return Result{Content: search.FormatResults(response.SearchResults)}, nil
}

func IsWebSearchTool(name string) bool {
	return name == WebSearchToolName
}

func RegisterSearchTools(registry *Registry, db *gorm.DB) {
	if registry == nil || db == nil {
		return
	}
	if !database.HasEnabledSearchConfig(db) {
		return
	}
	registry.Register(NewWebSearchTool(db))
}
