package session

import (
	"net/http"

	database "pkgs/db"
	"pkgs/httpclient"
)

// ensureLLMHTTPClient 返回任务级 tracing HTTP client（代理、超时、请求日志与主对话一致）。
// 同一 RuntimeConfig 内只构造一次，供主循环、标题生成、记忆压缩等旁路复用。
func (r *AgentRunner) ensureLLMHTTPClient(runtimeConfig *RuntimeConfig) *http.Client {
	if r == nil || runtimeConfig == nil {
		return nil
	}
	runtimeConfig.llmHTTPClientOnce.Do(func() {
		var proxyBase *http.Client
		if runtimeConfig.LLMConfig != nil {
			proxyBase = httpclient.ClientWithOptionalProxy(runtimeConfig.LLMConfig.Proxy)
		}
		runtimeConfig.LLMHTTPClient = newLLMTracingHTTPClient(proxyBase, llmTracingHooks{
			OnRequestStart: func(trace *llmAPICallTrace, requestBody string) {
				r.logLLMAPIRequest(runtimeConfig, "stream_chat_request", trace, requestBody)
			},
			OnResponseDone: func(trace *llmAPICallTrace, responseBody string, callErr error) {
				r.logLLMAPIResponse(runtimeConfig, "stream_chat_response", trace, responseBody, callErr)
			},
		}, database.GetLLMHTTPClientTimeout(r.db), database.GetLLMHTTPConnectTimeout(r.db))
	})
	return runtimeConfig.LLMHTTPClient
}

// ensureCompactionHTTPClient 为记忆压缩请求构造 HTTP client，代理与超时取自 compaction worker 的 LLM 配置。
func (r *AgentRunner) ensureCompactionHTTPClient(compactionRuntime *MemoryCompactionRuntime) *http.Client {
	if r == nil {
		return nil
	}
	var proxyBase *http.Client
	if compactionRuntime != nil && compactionRuntime.LLMConfig != nil {
		proxyBase = httpclient.ClientWithOptionalProxy(compactionRuntime.LLMConfig.Proxy)
	}
	return newLLMTracingHTTPClient(proxyBase, llmTracingHooks{
		OnRequestStart: func(trace *llmAPICallTrace, requestBody string) {
			r.logLLMAPIRequest(nil, "compaction_stream_chat_request", trace, requestBody)
		},
		OnResponseDone: func(trace *llmAPICallTrace, responseBody string, callErr error) {
			r.logLLMAPIResponse(nil, "compaction_stream_chat_response", trace, responseBody, callErr)
		},
	}, database.GetLLMHTTPClientTimeout(r.db), database.GetLLMHTTPConnectTimeout(r.db))
}
