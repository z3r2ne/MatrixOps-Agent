package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"pkgs/db/models"

	"github.com/google/uuid"
)

type Result struct {
	SiteName string `json:"site_name"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Snippet  string `json:"snippet"`
	Content  string `json:"content"`
	Date     string `json:"date"`
	Icon     string `json:"icon"`
	Mime     string `json:"mime"`
}

type Response struct {
	SearchResults []Result `json:"search_results"`
}

type Request struct {
	TextQuery            string `json:"text_query"`
	Limit                int    `json:"limit"`
	EnablePageCrawling   bool   `json:"enable_page_crawling"`
	TimeoutSeconds       int    `json:"timeout_seconds"`
}

func ResolveSearchEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = models.DefaultSearchConfigBaseURL
	}
	if strings.HasSuffix(baseURL, "/search") {
		return baseURL
	}
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/search"
	}
	return baseURL + "/v1/search"
}

func Search(config models.SearchConfig, query string, limit int, enablePageCrawling bool) (*Response, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("search api key is required")
	}
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	endpoint := ResolveSearchEndpoint(config.BaseURL)
	payload := Request{
		TextQuery:          query,
		Limit:                limit,
		EnablePageCrawling: enablePageCrawling,
		TimeoutSeconds:     30,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "matrixops/1.0")
	req.Header.Set("X-Msh-Tool-Call-Id", uuid.NewString())

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("search request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result Response
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse search response failed: %w", err)
	}
	return &result, nil
}

func FormatResults(results []Result) string {
	if len(results) == 0 {
		return "未找到相关搜索结果。"
	}
	var builder strings.Builder
	for index, item := range results {
		if index > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(fmt.Sprintf("[%d] %s\n", index+1, item.Title))
		if item.SiteName != "" {
			builder.WriteString(fmt.Sprintf("来源: %s\n", item.SiteName))
		}
		if item.URL != "" {
			builder.WriteString(fmt.Sprintf("链接: %s\n", item.URL))
		}
		if item.Date != "" {
			builder.WriteString(fmt.Sprintf("日期: %s\n", item.Date))
		}
		if item.Snippet != "" {
			builder.WriteString(fmt.Sprintf("摘要: %s\n", item.Snippet))
		}
		if item.Content != "" {
			builder.WriteString(fmt.Sprintf("内容: %s\n", item.Content))
		}
	}
	return builder.String()
}
