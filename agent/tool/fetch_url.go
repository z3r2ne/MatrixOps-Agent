package tool

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

const (
	FetchURLToolName       = "fetch_url"
	fetchURLMaxBodyBytes   = 10 * 1024 * 1024 // 10 MiB, aligned with Kimi FetchURL
	fetchURLDefaultTimeout = 30 * time.Second
	fetchURLMaxRedirects   = 10
)

type FetchURLTool struct{}

func (FetchURLTool) Name() string { return FetchURLToolName }

func (FetchURLTool) VerbosName() string { return "抓取网页" }

func (FetchURLTool) Description() string {
	return "从 URL 抓取页面并提取可读正文（HTML 会去掉脚本/样式）。仅支持 http/https；拒绝内网、回环、链路本地与元数据地址；响应体超过 10 MiB 会拒绝。"
}

func (FetchURLTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"url": map[string]interface{}{
			"type":        "string",
			"description": "要抓取的完整 URL，须以 http:// 或 https:// 开头",
		},
		"timeout_seconds": map[string]interface{}{
			"type":        "number",
			"description": "请求总超时（秒），默认 30，最大 120",
		},
	}, []string{"url"})
}

func (FetchURLTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	if err := CheckContext(ctx); err != nil {
		return Result{IsError: true, Name: FetchURLToolName}, err
	}

	rawURL, ok := input["url"].(string)
	if !ok {
		return Result{IsError: true, Name: FetchURLToolName}, errors.New("fetch_url: missing url")
	}

	parsed, err := validatePublicHTTPURL(rawURL)
	if err != nil {
		return Result{IsError: true, Name: FetchURLToolName}, fmt.Errorf("fetch_url: %w", err)
	}

	timeout := fetchURLDefaultTimeout
	if v, ok := input["timeout_seconds"]; ok && v != nil {
		if sec := floatFrom(v); sec > 0 {
			timeout = time.Duration(sec * float64(time.Second))
			if timeout > httpToolMaxTimeout {
				timeout = httpToolMaxTimeout
			}
		}
	}

	execCtx := ctx.Context
	if execCtx == nil {
		execCtx = context.Background()
	}

	req, err := http.NewRequestWithContext(execCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return Result{IsError: true, Name: FetchURLToolName}, err
	}
	req.Header.Set("User-Agent", "matrixops-agent-fetch-url/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,application/json;q=0.7,*/*;q=0.5")

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= fetchURLMaxRedirects {
				return errors.New("too many redirects")
			}
			if _, err := validatePublicHTTPURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return Result{IsError: true, Name: FetchURLToolName}, err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, fetchURLMaxBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return Result{IsError: true, Name: FetchURLToolName}, err
	}
	if len(body) > fetchURLMaxBodyBytes {
		return Result{IsError: true, Name: FetchURLToolName}, fmt.Errorf("fetch_url: response body exceeds %d bytes", fetchURLMaxBodyBytes)
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	mediaType, _, _ := mime.ParseMediaType(contentType)

	text, extractErr := extractFetchedBody(body, mediaType)
	if extractErr != nil {
		return Result{IsError: true, Name: FetchURLToolName}, extractErr
	}

	var out strings.Builder
	fmt.Fprintf(&out, "URL: %s\n", parsed.String())
	if resp.Request != nil && resp.Request.URL != nil && resp.Request.URL.String() != parsed.String() {
		fmt.Fprintf(&out, "Final URL: %s\n", resp.Request.URL.String())
	}
	fmt.Fprintf(&out, "Status: %d\n", resp.StatusCode)
	if contentType != "" {
		fmt.Fprintf(&out, "Content-Type: %s\n", contentType)
	}
	out.WriteString("\n")
	out.WriteString(text)

	return Result{
		Name:    FetchURLToolName,
		Content: out.String(),
		Metadata: map[string]interface{}{
			"url":         parsed.String(),
			"status":      resp.StatusCode,
			"contentType": contentType,
		},
	}, nil
}

func extractFetchedBody(body []byte, mediaType string) (string, error) {
	if len(body) == 0 {
		return "", nil
	}

	lower := strings.ToLower(mediaType)
	switch {
	case strings.HasPrefix(lower, "text/html"), strings.HasPrefix(lower, "application/xhtml"):
		text, err := extractTextFromHTML(body)
		if err != nil {
			return "", fmt.Errorf("fetch_url: parse html: %w", err)
		}
		if text == "" {
			return "", errors.New("fetch_url: no extractable text in html response")
		}
		return text, nil
	case strings.HasPrefix(lower, "text/"), strings.HasPrefix(lower, "application/json"),
		strings.HasPrefix(lower, "application/xml"), strings.HasPrefix(lower, "application/javascript"),
		strings.HasPrefix(lower, "application/x-javascript"):
		return string(body), nil
	default:
		if isLikelyHTML(body) {
			text, err := extractTextFromHTML(body)
			if err == nil && text != "" {
				return text, nil
			}
		}
		return "", fmt.Errorf("fetch_url: unsupported content type %q (only text/html and text-based responses are supported)", mediaType)
	}
}

func isLikelyHTML(body []byte) bool {
	trimmed := strings.TrimSpace(string(body))
	if len(trimmed) > 512 {
		trimmed = trimmed[:512]
	}
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html") || strings.Contains(lower, "<body")
}
