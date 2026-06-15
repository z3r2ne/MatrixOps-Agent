package anthropic

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"

	"pkgs/db/models"
	"pkgs/httpclient"
	"pkgs/llmheaders"

	"matrixops.local/core_agent/streamtypes"

	"github.com/anthropics/anthropic-sdk-go/option"
)

type anthropicTransportState struct {
	rawBodyResponse string
	responseStatus  int
	responseHeaders map[string]string
}

func anthropicEventStreamContentType(contentType string) bool {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		return strings.EqualFold(mediaType, "text/event-stream")
	}
	return strings.Contains(strings.ToLower(contentType), "text/event-stream")
}

func anthropicResponseHeaders(header http.Header) map[string]string {
	if len(header) == 0 {
		return nil
	}
	out := make(map[string]string, len(header))
	for key, values := range header {
		if len(values) == 0 {
			continue
		}
		out[strings.ToLower(key)] = values[0]
	}
	return out
}

type rawResponseCaptureReadCloser struct {
	inner    io.ReadCloser
	callback func(string)

	mu     sync.Mutex
	buf    bytes.Buffer
	closed bool
}

func newRawResponseCaptureReadCloser(inner io.ReadCloser, callback func(string)) io.ReadCloser {
	if inner == nil {
		return nil
	}
	return &rawResponseCaptureReadCloser{
		inner:    inner,
		callback: callback,
	}
}

func (r *rawResponseCaptureReadCloser) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	if n > 0 {
		r.mu.Lock()
		_, _ = r.buf.Write(p[:n])
		r.mu.Unlock()
	}
	if err == io.EOF {
		r.emit()
	}
	return n, err
}

func (r *rawResponseCaptureReadCloser) Close() error {
	err := r.inner.Close()
	r.emit()
	return err
}

func (r *rawResponseCaptureReadCloser) emit() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	payload := r.buf.String()
	callback := r.callback
	r.mu.Unlock()

	if callback != nil {
		callback(payload)
	}
}

func buildAnthropicClientOptions(input streamtypes.StreamInput, llm *models.LLMConfig) (*anthropicTransportState, []option.RequestOption, error) {
	state := &anthropicTransportState{}
	apiKey := strings.TrimSpace(llm.APIKey)
	if apiKey == "" {
		return nil, nil, fmt.Errorf("anthropic native tools: API key is empty")
	}
	opts := []option.RequestOption{
		option.WithoutEnvironmentDefaults(),
		option.WithAPIKey(apiKey),
		option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
			if req != nil {
				req.Header.Del("User-Agent")
			}
			return next(req)
		}),
		option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
			if req != nil {
				llmheaders.Apply(req.Header)
			}
			return next(req)
		}),
	}
	switch {
	case input.HTTPClient != nil:
		opts = append(opts, option.WithHTTPClient(input.HTTPClient))
	default:
		if p := strings.TrimSpace(llm.Proxy); p != "" {
			if pc := httpclient.ClientWithOptionalProxy(p); pc != nil {
				opts = append(opts, option.WithHTTPClient(pc))
			}
		}
	}
	if base := strings.TrimSpace(llm.BaseURL); base != "" {
		opts = append(opts, option.WithBaseURL(strings.TrimSuffix(base, "/")))
	}
	if input.OnRawRequest != nil {
		opts = append(opts, option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
			if req != nil && req.Body != nil {
				body, err := io.ReadAll(req.Body)
				_ = req.Body.Close()
				if err != nil {
					return nil, err
				}
				req.Body = io.NopCloser(bytes.NewReader(body))
				input.OnRawRequest(string(body))
			}
			return next(req)
		}))
	}
	opts = append(opts, option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		resp, err := next(req)
		if err != nil || resp == nil || resp.Body == nil {
			return resp, err
		}
		state.responseStatus = resp.StatusCode
		state.responseHeaders = anthropicResponseHeaders(resp.Header)
		contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
		if contentType != "" && !anthropicEventStreamContentType(contentType) {
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				return nil, readErr
			}
			state.rawBodyResponse = string(body)
			if input.OnRawResponse != nil {
				input.OnRawResponse(state.rawBodyResponse)
			}
			return nil, anthropicNoEventStreamError(llm, state.responseStatus, state.responseHeaders, fmt.Sprintf("unexpected streaming content-type %q", contentType), state.rawBodyResponse)
		}
		resp.Body = newRawResponseCaptureReadCloser(resp.Body, func(raw string) {
			state.rawBodyResponse = raw
			if input.OnRawResponse != nil {
				input.OnRawResponse(raw)
			}
		})
		return resp, err
	}))
	return state, opts, nil
}

func anthropicNoEventStreamError(llm *models.LLMConfig, statusCode int, headers map[string]string, reason string, rawResponse string) error {
	rawResponse = strings.TrimSpace(rawResponse)
	if rawResponse == "" && statusCode == 0 {
		return nil
	}
	msg := strings.TrimSpace(reason)
	if msg == "" {
		msg = "anthropic native stream ended without any events"
	}
	if streamtypes.RawResponseLooksLikeRetryableProxyHTML(rawResponse) {
		msg = "anthropic native stream returned proxy/html error page before any events"
	}
	return anthropicRetryableStreamError(llm, statusCode, headers, msg, rawResponse)
}
