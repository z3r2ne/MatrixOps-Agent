package openai_native

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"pkgs/db/models"
	"pkgs/httpclient"
	"pkgs/llmheaders"

	"matrixops.local/core_agent/streamtypes"

	"github.com/openai/openai-go/option"
)
type openAINativeTransportState struct {
	rawBodyResponse string
	responseStatus  int
	responseHeaders map[string]string
}

func buildOpenAINativeRequestOptions(input streamtypes.StreamInput, llm *models.LLMConfig) (*openAINativeTransportState, []option.RequestOption, error) {
	state := &openAINativeTransportState{}
	apiKey := strings.TrimSpace(llm.APIKey)
	if apiKey == "" {
		return nil, nil, fmt.Errorf("openai native tools: API key is empty")
	}
	opts := []option.RequestOption{
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
		state.responseHeaders = openAINativeResponseHeaders(resp.Header)
		contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
		if contentType != "" && !openAINativeEventStreamContentType(contentType) {
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				return nil, readErr
			}
			state.rawBodyResponse = string(body)
			if input.OnRawResponse != nil {
				input.OnRawResponse(state.rawBodyResponse)
			}
			return nil, openAINativeRetryableStreamError(llm, state.responseStatus, state.responseHeaders, fmt.Sprintf("unexpected streaming content-type %q", contentType), state.rawBodyResponse)
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
