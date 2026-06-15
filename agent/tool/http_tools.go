package tool

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	httpToolMaxBodyBytes   = 512 * 1024
	httpToolDefaultTimeout = 30 * time.Second
	httpToolMaxTimeout     = 120 * time.Second
)

type HTTPHeadTool struct{}

func (HTTPHeadTool) Name() string { return "http_head" }

func (HTTPHeadTool) VerbosName() string { return "HTTP HEAD" }

func (HTTPHeadTool) Description() string {
	return "对给定 URL 发起 HTTP HEAD 请求，返回状态行与响应头（不含响应体）。仅支持 http/https。"
}

func (HTTPHeadTool) Schema() map[string]interface{} {
	return httpToolSchema(false)
}

func (HTTPHeadTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	return executeSimpleHTTP(ctx, input, http.MethodHead, false)
}

type HTTPDeleteTool struct{}

func (HTTPDeleteTool) Name() string { return "http_delete" }

func (HTTPDeleteTool) VerbosName() string { return "HTTP DELETE" }

func (HTTPDeleteTool) Description() string {
	return "对给定 URL 发起 HTTP DELETE 请求，返回状态、响应头与响应体（体长度有上限）。仅支持 http/https；如需 JSON 等请求体请在 headers 中设置 Content-Type。"
}

func (HTTPDeleteTool) Schema() map[string]interface{} {
	return httpToolSchema(true)
}

func (HTTPDeleteTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	return executeSimpleHTTP(ctx, input, http.MethodDelete, true)
}

func httpToolSchema(includeBody bool) map[string]interface{} {
	props := map[string]interface{}{
		"url": map[string]interface{}{
			"type":        "string",
			"description": "完整 URL，须以 http:// 或 https:// 开头",
		},
		"headers": map[string]interface{}{
			"type":                 "object",
			"description":          "可选请求头；键值均为字符串",
			"additionalProperties": map[string]interface{}{"type": "string"},
		},
		"timeout_seconds": map[string]interface{}{
			"type":        "number",
			"description": "请求总超时（秒），默认 30，最大 120",
		},
	}
	required := []string{"url"}
	if includeBody {
		props["body"] = map[string]interface{}{
			"type":        "string",
			"description": "可选请求体原始字符串（如 JSON 文本）",
		}
	}
	return ObjectParamSchema(props, required)
}

func executeSimpleHTTP(ctx Context, input map[string]interface{}, method string, allowBody bool) (Result, error) {
	rawURL, ok := input["url"].(string)
	if !ok {
		return Result{IsError: true, Name: toolNameForMethod(method)}, errors.New("missing url")
	}
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return Result{IsError: true, Name: toolNameForMethod(method)}, errors.New("missing url")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return Result{IsError: true, Name: toolNameForMethod(method)}, fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return Result{IsError: true, Name: toolNameForMethod(method)}, errors.New("only http and https URLs are allowed")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return Result{IsError: true, Name: toolNameForMethod(method)}, errors.New("url host is empty")
	}

	timeout := httpToolDefaultTimeout
	if v, ok := input["timeout_seconds"]; ok && v != nil {
		if sec := floatFrom(v); sec > 0 {
			timeout = time.Duration(sec * float64(time.Second))
			if timeout > httpToolMaxTimeout {
				timeout = httpToolMaxTimeout
			}
		}
	}

	var bodyReader io.Reader
	if allowBody {
		if b, ok := input["body"].(string); ok && b != "" {
			bodyReader = strings.NewReader(b)
		}
	}

	execCtx := ctx.Context
	if execCtx == nil {
		execCtx = context.Background()
	}

	req, err := http.NewRequestWithContext(execCtx, method, rawURL, bodyReader)
	if err != nil {
		return Result{IsError: true, Name: toolNameForMethod(method)}, err
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "matrixops-agent-http-tool/1.0")
	}
	if hdrs, ok := input["headers"].(map[string]interface{}); ok {
		for k, v := range hdrs {
			key := strings.TrimSpace(k)
			if key == "" {
				continue
			}
			val, sok := v.(string)
			if !sok {
				val = fmt.Sprint(v)
			}
			req.Header.Set(key, val)
		}
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return Result{IsError: true, Name: toolNameForMethod(method)}, err
	}
	defer resp.Body.Close()

	var body string
	if allowBody && resp.Body != nil {
		limited := io.LimitReader(resp.Body, httpToolMaxBodyBytes+1)
		data, readErr := io.ReadAll(limited)
		if readErr != nil {
			return Result{IsError: true, Name: toolNameForMethod(method)}, readErr
		}
		truncatedNote := ""
		if len(data) > httpToolMaxBodyBytes {
			data = data[:httpToolMaxBodyBytes]
			truncatedNote = fmt.Sprintf("\n\n[truncated: response body exceeded %d bytes]", httpToolMaxBodyBytes)
		}
		body = string(data) + truncatedNote
	} else if resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
	}

	out := strings.Builder{}
	fmt.Fprintf(&out, "%s %s\n", resp.Proto, resp.Status)
	out.WriteString(formatSortedHeaders(resp.Header))
	if allowBody && body != "" {
		out.WriteString("\n")
		out.WriteString(body)
	}

	name := toolNameForMethod(method)
	return Result{
		Name:    name,
		Content: out.String(),
		Metadata: map[string]interface{}{
			"url":    rawURL,
			"method": method,
			"status": resp.StatusCode,
		},
	}, nil
}

func toolNameForMethod(method string) string {
	switch method {
	case http.MethodHead:
		return "http_head"
	case http.MethodDelete:
		return "http_delete"
	default:
		return "http"
	}
}

func floatFrom(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func formatSortedHeaders(h http.Header) string {
	if len(h) == 0 {
		return ""
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		for _, v := range h[k] {
			fmt.Fprintf(&b, "%s: %s\n", k, v)
		}
	}
	return b.String()
}
