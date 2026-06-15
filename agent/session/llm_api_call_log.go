package session

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	database "pkgs/db"
	"pkgs/db/models"
)

const (
	llmAPICallSource       = "llm_api_call"
	maxLLMLogPayloadLength = 200_000
)

var llmAPICallTraceSeq uint64

type llmAPICallTrace struct {
	mu sync.Mutex

	requestID string

	startedAt  time.Time
	finishedAt time.Time

	method       string
	url          string
	statusCode   int
	requestSize  int64
	responseSize int64

	dnsStartAt     time.Time
	connectStartAt time.Time
	tlsStartAt     time.Time
	wroteAt        time.Time
	firstByteAt    time.Time

	dnsDuration     time.Duration
	connectDuration time.Duration
	tlsDuration     time.Duration
	firstByte       time.Duration
	serverWait      time.Duration
	totalDuration   time.Duration

	connReused  bool
	connWasIdle bool
	connIdleFor time.Duration
	remoteAddr  string

	errText string
}

type llmAPICallSnapshot struct {
	RequestID       string
	StartedAt       time.Time
	FinishedAt      time.Time
	Method          string
	URL             string
	StatusCode      int
	RequestSize     int64
	ResponseSize    int64
	DNSDuration     time.Duration
	ConnectDuration time.Duration
	TLSDuration     time.Duration
	FirstByte       time.Duration
	ServerWait      time.Duration
	TotalDuration   time.Duration
	ConnReused      bool
	ConnWasIdle     bool
	ConnIdleFor     time.Duration
	RemoteAddr      string
	Error           string
}

type llmTracingHooks struct {
	OnRequestStart func(trace *llmAPICallTrace, requestBody string)
	OnResponseDone func(trace *llmAPICallTrace, responseBody string, err error)
}

type llmTracingRoundTripper struct {
	base  http.RoundTripper
	hooks llmTracingHooks
}

type llmTracingReadCloser struct {
	io.ReadCloser
	trace      *llmAPICallTrace
	onComplete func(trace *llmAPICallTrace, responseBody string, err error)

	mu           sync.Mutex
	buf          bytes.Buffer
	completeOnce sync.Once
}

func newLLMTracingHTTPClient(base *http.Client, hooks llmTracingHooks, requestTimeout time.Duration, connectTimeout time.Duration) *http.Client {
	var client *http.Client
	if base != nil {
		cloned := *base
		client = &cloned
	} else {
		client = &http.Client{}
	}
	if requestTimeout > 0 {
		client.Timeout = requestTimeout
	}

	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	if connectTimeout > 0 {
		if t, ok := transport.(*http.Transport); ok {
			clonedTransport := t.Clone()
			clonedTransport.ResponseHeaderTimeout = connectTimeout
			transport = clonedTransport
		}
	}
	client.Transport = &llmTracingRoundTripper{
		base:  transport,
		hooks: hooks,
	}

	return client
}

func (rt *llmTracingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	trace := &llmAPICallTrace{}
	requestBody, err := captureLLMRequestBody(req)
	if err != nil {
		trace.begin(req, 0)
		trace.finish(err)
		if rt.hooks.OnResponseDone != nil {
			rt.hooks.OnResponseDone(trace, "", err)
		}
		return nil, err
	}

	trace.begin(req, int64(len(requestBody)))
	if rt.hooks.OnRequestStart != nil {
		rt.hooks.OnRequestStart(trace, string(requestBody))
	}

	traceCtx := httptrace.WithClientTrace(req.Context(), trace.clientTrace())
	clonedReq := req.Clone(traceCtx)
	clonedReq.Body = req.Body

	resp, err := rt.base.RoundTrip(clonedReq)
	if err != nil {
		trace.finish(err)
		if rt.hooks.OnResponseDone != nil {
			rt.hooks.OnResponseDone(trace, "", err)
		}
		return nil, err
	}

	trace.markResponse(resp)
	if resp.Body == nil {
		trace.finish(nil)
		if rt.hooks.OnResponseDone != nil {
			rt.hooks.OnResponseDone(trace, "", nil)
		}
		return resp, nil
	}

	resp.Body = &llmTracingReadCloser{
		ReadCloser: resp.Body,
		trace:      trace,
		onComplete: rt.hooks.OnResponseDone,
	}
	return resp, nil
}

func (rc *llmTracingReadCloser) Read(p []byte) (int, error) {
	n, err := rc.ReadCloser.Read(p)
	if n > 0 && rc.trace != nil {
		rc.trace.addResponseBytes(n)
		rc.trace.markFirstByteRead()
		rc.appendCaptured(p[:n])
	}
	if err == io.EOF && rc.trace != nil {
		rc.complete(nil)
	}
	return n, err
}

func (rc *llmTracingReadCloser) Close() error {
	err := rc.ReadCloser.Close()
	if rc.trace != nil {
		rc.complete(err)
	}
	return err
}

func (rc *llmTracingReadCloser) appendCaptured(chunk []byte) {
	if len(chunk) == 0 {
		return
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	remaining := maxLLMLogPayloadLength + 1 - rc.buf.Len()
	if remaining <= 0 {
		return
	}
	if len(chunk) > remaining {
		chunk = chunk[:remaining]
	}
	_, _ = rc.buf.Write(chunk)
}

func (rc *llmTracingReadCloser) complete(err error) {
	rc.completeOnce.Do(func() {
		if rc.trace != nil {
			rc.trace.finish(err)
		}
		if rc.onComplete == nil {
			return
		}

		rc.mu.Lock()
		responseBody := rc.buf.String()
		rc.mu.Unlock()

		rc.onComplete(rc.trace, responseBody, err)
	})
}

func captureLLMRequestBody(req *http.Request) ([]byte, error) {
	if req == nil || req.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func (t *llmAPICallTrace) begin(req *http.Request, requestSize int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.requestID == "" {
		t.requestID = fmt.Sprintf("llmreq-%d", atomic.AddUint64(&llmAPICallTraceSeq, 1))
	}
	if t.startedAt.IsZero() {
		t.startedAt = time.Now()
	}
	t.method = req.Method
	if req.URL != nil {
		t.url = req.URL.Redacted()
	}
	if requestSize > 0 {
		t.requestSize = requestSize
	} else if req.ContentLength > 0 {
		t.requestSize = req.ContentLength
	}
}

func (t *llmAPICallTrace) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()

			t.connReused = info.Reused
			t.connWasIdle = info.WasIdle
			t.connIdleFor = info.IdleTime
			if info.Conn != nil {
				t.remoteAddr = info.Conn.RemoteAddr().String()
			}
		},
		DNSStart: func(httptrace.DNSStartInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.dnsStartAt = time.Now()
		},
		DNSDone: func(httptrace.DNSDoneInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			if !t.dnsStartAt.IsZero() {
				t.dnsDuration = time.Since(t.dnsStartAt)
			}
		},
		ConnectStart: func(string, string) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.connectStartAt = time.Now()
		},
		ConnectDone: func(string, string, error) {
			t.mu.Lock()
			defer t.mu.Unlock()
			if !t.connectStartAt.IsZero() {
				t.connectDuration = time.Since(t.connectStartAt)
			}
		},
		TLSHandshakeStart: func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.tlsStartAt = time.Now()
		},
		TLSHandshakeDone: func(tls.ConnectionState, error) {
			t.mu.Lock()
			defer t.mu.Unlock()
			if !t.tlsStartAt.IsZero() {
				t.tlsDuration = time.Since(t.tlsStartAt)
			}
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.wroteAt = time.Now()
		},
		GotFirstResponseByte: func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			if t.firstByteAt.IsZero() {
				now := time.Now()
				t.firstByteAt = now
				if !t.startedAt.IsZero() {
					t.firstByte = now.Sub(t.startedAt)
				}
				if !t.wroteAt.IsZero() {
					t.serverWait = now.Sub(t.wroteAt)
				}
			}
		},
	}
}

func (t *llmAPICallTrace) markResponse(resp *http.Response) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.statusCode = resp.StatusCode
}

func (t *llmAPICallTrace) addResponseBytes(n int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.responseSize += int64(n)
}

func (t *llmAPICallTrace) markFirstByteRead() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.firstByteAt.IsZero() {
		now := time.Now()
		t.firstByteAt = now
		if !t.startedAt.IsZero() {
			t.firstByte = now.Sub(t.startedAt)
		}
		if !t.wroteAt.IsZero() {
			t.serverWait = now.Sub(t.wroteAt)
		}
	}
}

func (t *llmAPICallTrace) finish(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.finishedAt.IsZero() {
		if err != nil && t.errText == "" {
			t.errText = err.Error()
		}
		return
	}

	now := time.Now()
	t.finishedAt = now
	if !t.startedAt.IsZero() {
		t.totalDuration = now.Sub(t.startedAt)
	}
	if err != nil && t.errText == "" {
		t.errText = err.Error()
	}
}

func (t *llmAPICallTrace) snapshot() llmAPICallSnapshot {
	if t == nil {
		return llmAPICallSnapshot{}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	return llmAPICallSnapshot{
		RequestID:       t.requestID,
		StartedAt:       t.startedAt,
		FinishedAt:      t.finishedAt,
		Method:          t.method,
		URL:             t.url,
		StatusCode:      t.statusCode,
		RequestSize:     t.requestSize,
		ResponseSize:    t.responseSize,
		DNSDuration:     t.dnsDuration,
		ConnectDuration: t.connectDuration,
		TLSDuration:     t.tlsDuration,
		FirstByte:       t.firstByte,
		ServerWait:      t.serverWait,
		TotalDuration:   t.totalDuration,
		ConnReused:      t.connReused,
		ConnWasIdle:     t.connWasIdle,
		ConnIdleFor:     t.connIdleFor,
		RemoteAddr:      t.remoteAddr,
		Error:           t.errText,
	}
}

func (t *llmAPICallTrace) hasActivity() bool {
	snapshot := t.snapshot()
	return !snapshot.StartedAt.IsZero() || snapshot.Method != "" || snapshot.URL != ""
}

func (r *AgentRunner) logLLMAPIRequest(runtimeConfig *RuntimeConfig, command string, trace *llmAPICallTrace, rawRequest string) {
	if r.db == nil || trace == nil {
		return
	}

	snapshot := trace.snapshot()
	if !trace.hasActivity() && rawRequest == "" {
		return
	}

	startedAt := snapshot.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	finishedAt := startedAt

	requestSize := snapshot.RequestSize
	if requestSize <= 0 && rawRequest != "" {
		requestSize = int64(len(rawRequest))
	}

	argsJSON, _ := json.Marshal(buildLLMAPIRequestArgs(runtimeConfig, snapshot, requestSize))

	logEntry := &models.CommandLog{
		Source:     llmAPICallSource,
		SourceName: buildLLMAPICallSourceName(r, runtimeConfig),
		Command:    command,
		Args:       string(argsJSON),
		WorkDir:    r.GetDirectory(),
		// 原始请求完整入库，便于日志页展示与排查；响应体仍可能很大故继续截断
		StdinData: rawRequest,
		Fields: models.BuildCommandLogFields(
			models.NewCommandLogField("request_id", "请求 ID", snapshot.RequestID, "default"),
			models.NewCommandLogField("raw_request", "原始请求报文", rawRequest, "default"),
		),
		Duration:   0,
		Status:     "success",
		CreatedAt:  startedAt,
		FinishedAt: &finishedAt,
	}
	if r.task != nil {
		logEntry.SourceID = &r.task.ID
	}
	if err := database.CreateCommandLog(r.db, logEntry); err != nil {
		return
	}
}

func (r *AgentRunner) logLLMAPIResponse(runtimeConfig *RuntimeConfig, command string, trace *llmAPICallTrace, rawResponse string, callErr error) {
	if r.db == nil || trace == nil {
		return
	}

	snapshot := trace.snapshot()
	if !trace.hasActivity() && rawResponse == "" && callErr == nil {
		return
	}

	startedAt := snapshot.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	finishedAt := snapshot.FinishedAt
	if finishedAt.IsZero() {
		finishedAt = time.Now()
	}

	duration := snapshot.TotalDuration
	if duration <= 0 {
		duration = finishedAt.Sub(startedAt)
		if duration < 0 {
			duration = 0
		}
	}

	requestSize := snapshot.RequestSize
	responseSize := snapshot.ResponseSize
	if responseSize <= 0 && rawResponse != "" {
		responseSize = int64(len(rawResponse))
	}

	argsJSON, _ := json.Marshal(buildLLMAPIResponseArgs(runtimeConfig, snapshot, requestSize, responseSize))

	errorText := snapshot.Error
	if callErr != nil {
		errorText = callErr.Error()
		if snapshot.Error != "" && snapshot.Error != callErr.Error() {
			errorText = fmt.Sprintf("%s\ntransport=%s", errorText, snapshot.Error)
		}
	}

	status := "success"
	if callErr != nil || snapshot.StatusCode >= 400 {
		status = "failed"
	}

	logEntry := &models.CommandLog{
		Source:     llmAPICallSource,
		SourceName: buildLLMAPICallSourceName(r, runtimeConfig),
		Command:    command,
		Args:       string(argsJSON),
		WorkDir:    r.GetDirectory(),
		Stdout:     truncateLLMLogPayload(rawResponse),
		Fields: models.BuildCommandLogFields(
			models.NewCommandLogField("request_id", "请求 ID", snapshot.RequestID, "default"),
			models.NewCommandLogField("raw_response", "响应报文", truncateLLMLogPayload(rawResponse), "default"),
			models.NewCommandLogField("error", "错误信息", errorText, "error"),
		),
		Error:      errorText,
		Duration:   duration.Milliseconds(),
		Status:     status,
		CreatedAt:  startedAt,
		FinishedAt: &finishedAt,
	}
	if r.task != nil {
		logEntry.SourceID = &r.task.ID
	}
	if err := database.CreateCommandLog(r.db, logEntry); err != nil {
		return
	}
}

func buildLLMActionsJSON(actions []string) string {
	if len(actions) == 0 {
		return ""
	}
	values := make([]any, 0, len(actions))
	for _, action := range actions {
		action = strings.TrimSpace(action)
		if action == "" {
			continue
		}
		var value any
		if err := json.Unmarshal([]byte(action), &value); err != nil {
			values = append(values, action)
			continue
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return ""
	}
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return ""
	}
	return string(data)
}

func (r *AgentRunner) logLLMAPIAttempt(runtimeConfig *RuntimeConfig, command string, rawRequest string, rawResponse string, callErr error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration) {
	if r.db == nil || callErr == nil {
		return
	}

	startedAt := time.Now()
	if attemptDuration > 0 {
		startedAt = startedAt.Add(-attemptDuration)
	}
	finishedAt := time.Now()

	argsJSON, _ := json.Marshal(buildLLMAPIAttemptArgs(runtimeConfig, retryAttempt, maxRetries, nextDelay, attemptDuration, callErr))

	errorText := callErr.Error()
	logEntry := &models.CommandLog{
		Source:     llmAPICallSource,
		SourceName: buildLLMAPICallSourceName(r, runtimeConfig),
		Command:    command,
		Args:       string(argsJSON),
		WorkDir:    r.GetDirectory(),
		StdinData:  truncateLLMLogPayload(rawRequest),
		Stdout:     truncateLLMLogPayload(rawResponse),
		Fields:     models.LegacyCommandLogFields(truncateLLMLogPayload(rawRequest), truncateLLMLogPayload(rawResponse), "", errorText),
		Error:      errorText,
		Duration:   attemptDuration.Milliseconds(),
		Status:     "failed",
		CreatedAt:  startedAt,
		FinishedAt: &finishedAt,
	}
	if r.task != nil {
		logEntry.SourceID = &r.task.ID
	}
	_ = database.CreateCommandLog(r.db, logEntry)
}

func buildLLMAPICallSourceName(r *AgentRunner, runtimeConfig *RuntimeConfig) string {
	sessionID := ""
	if r != nil {
		sessionID = r.GetSessionID()
	}

	providerName := ""
	modelName := ""
	if runtimeConfig != nil {
		if runtimeConfig.LLMConfig != nil {
			providerName = runtimeConfig.LLMConfig.Name
		}
		modelName = runtimeConfig.Model
		if modelName == "" && runtimeConfig.Worker != nil {
			modelName = runtimeConfig.Worker.Model
		}
	}

	switch {
	case sessionID != "" && providerName != "" && modelName != "":
		return fmt.Sprintf("Session %s · %s/%s", sessionID, providerName, modelName)
	case sessionID != "" && providerName != "":
		return fmt.Sprintf("Session %s · %s", sessionID, providerName)
	case sessionID != "":
		return fmt.Sprintf("Session %s", sessionID)
	case providerName != "" && modelName != "":
		return fmt.Sprintf("%s/%s", providerName, modelName)
	default:
		return "LLM API"
	}
}

func buildLLMAPICommonArgs(runtimeConfig *RuntimeConfig, snapshot llmAPICallSnapshot) []string {
	args := make([]string, 0, 18)

	if snapshot.RequestID != "" {
		args = append(args, fmt.Sprintf("request_id=%s", sanitizeLLMLogArg(snapshot.RequestID)))
	}
	if runtimeConfig != nil && runtimeConfig.LLMConfig != nil && runtimeConfig.LLMConfig.Name != "" {
		args = append(args, fmt.Sprintf("provider=%s", runtimeConfig.LLMConfig.Name))
	}
	if runtimeConfig != nil {
		modelName := runtimeConfig.Model
		if modelName == "" && runtimeConfig.Worker != nil {
			modelName = runtimeConfig.Worker.Model
		}
		if modelName != "" {
			args = append(args, fmt.Sprintf("model=%s", modelName))
		}
	}
	if snapshot.Method != "" {
		args = append(args, fmt.Sprintf("method=%s", snapshot.Method))
	}
	if snapshot.URL != "" {
		args = append(args, fmt.Sprintf("url=%s", snapshot.URL))
	}

	return args
}

func buildLLMAPIRequestArgs(runtimeConfig *RuntimeConfig, snapshot llmAPICallSnapshot, requestSize int64) []string {
	args := buildLLMAPICommonArgs(runtimeConfig, snapshot)
	args = append(args, "phase=request")
	if requestSize > 0 {
		args = append(args, fmt.Sprintf("request_bytes=%d", requestSize))
	}
	return args
}

func buildLLMAPIResponseArgs(runtimeConfig *RuntimeConfig, snapshot llmAPICallSnapshot, requestSize int64, responseSize int64) []string {
	args := buildLLMAPICommonArgs(runtimeConfig, snapshot)
	args = append(args, "phase=response")
	if snapshot.StatusCode > 0 {
		args = append(args, fmt.Sprintf("status_code=%d", snapshot.StatusCode))
	}
	if requestSize > 0 {
		args = append(args, fmt.Sprintf("request_bytes=%d", requestSize))
	}
	if responseSize > 0 {
		args = append(args, fmt.Sprintf("response_bytes=%d", responseSize))
	}
	if snapshot.DNSDuration > 0 {
		args = append(args, fmt.Sprintf("dns_ms=%d", snapshot.DNSDuration.Milliseconds()))
	}
	if snapshot.ConnectDuration > 0 {
		args = append(args, fmt.Sprintf("connect_ms=%d", snapshot.ConnectDuration.Milliseconds()))
	}
	if snapshot.TLSDuration > 0 {
		args = append(args, fmt.Sprintf("tls_handshake_ms=%d", snapshot.TLSDuration.Milliseconds()))
	}
	if snapshot.FirstByte > 0 {
		args = append(args, fmt.Sprintf("first_byte_ms=%d", snapshot.FirstByte.Milliseconds()))
	}
	if snapshot.ServerWait > 0 {
		args = append(args, fmt.Sprintf("server_wait_ms=%d", snapshot.ServerWait.Milliseconds()))
	}
	if snapshot.TotalDuration > 0 {
		args = append(args, fmt.Sprintf("total_ms=%d", snapshot.TotalDuration.Milliseconds()))
	}
	args = append(args, fmt.Sprintf("conn_reused=%t", snapshot.ConnReused))
	args = append(args, fmt.Sprintf("conn_was_idle=%t", snapshot.ConnWasIdle))
	if snapshot.ConnIdleFor > 0 {
		args = append(args, fmt.Sprintf("conn_idle_ms=%d", snapshot.ConnIdleFor.Milliseconds()))
	}
	if snapshot.RemoteAddr != "" {
		args = append(args, fmt.Sprintf("remote_addr=%s", snapshot.RemoteAddr))
	}

	return args
}

func buildLLMAPIAttemptArgs(runtimeConfig *RuntimeConfig, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, callErr error) []string {
	args := make([]string, 0, 12)

	if runtimeConfig != nil && runtimeConfig.LLMConfig != nil && runtimeConfig.LLMConfig.Name != "" {
		args = append(args, fmt.Sprintf("provider=%s", runtimeConfig.LLMConfig.Name))
	}
	if runtimeConfig != nil {
		modelName := runtimeConfig.Model
		if modelName == "" && runtimeConfig.Worker != nil {
			modelName = runtimeConfig.Worker.Model
		}
		if modelName != "" {
			args = append(args, fmt.Sprintf("model=%s", modelName))
		}
	}

	args = append(args, "attempt_log=true")
	if retryAttempt > 0 {
		args = append(args, fmt.Sprintf("attempt=%d", retryAttempt))
	}
	if maxRetries > 0 {
		args = append(args, fmt.Sprintf("max_retries=%d", maxRetries))
	}
	if nextDelay > 0 {
		args = append(args, fmt.Sprintf("next_retry_ms=%d", nextDelay.Milliseconds()))
	}
	if attemptDuration > 0 {
		args = append(args, fmt.Sprintf("attempt_ms=%d", attemptDuration.Milliseconds()))
	}

	if messageError := FromError(callErr, ""); messageError != nil {
		if messageError.StatusCode > 0 {
			args = append(args, fmt.Sprintf("status_code=%d", messageError.StatusCode))
		}
		if messageError.Name != "" {
			args = append(args, fmt.Sprintf("error_name=%s", messageError.Name))
		}
		if reason := extractRetryReason(messageError, callErr); reason != "" {
			args = append(args, fmt.Sprintf("reason=%s", sanitizeLLMLogArg(reason)))
		}
	} else if callErr != nil {
		args = append(args, fmt.Sprintf("reason=%s", sanitizeLLMLogArg(callErr.Error())))
	}

	return args
}

func extractRetryReason(messageError *MessageError, callErr error) string {
	if messageError != nil {
		if messageError.Metadata != nil && strings.TrimSpace(messageError.Metadata["reason"]) != "" {
			return strings.TrimSpace(messageError.Metadata["reason"])
		}
		if strings.TrimSpace(messageError.ResponseBody) != "" {
			return strings.TrimSpace(messageError.ResponseBody)
		}
		if strings.TrimSpace(messageError.Message) != "" {
			return strings.TrimSpace(messageError.Message)
		}
	}
	if callErr != nil {
		return strings.TrimSpace(callErr.Error())
	}
	return ""
}

func sanitizeLLMLogArg(value string) string {
	replacer := strings.NewReplacer("\n", "\\n", "\r", "\\r", "\t", "\\t")
	return replacer.Replace(value)
}

func truncateLLMLogPayload(payload string) string {
	if len(payload) <= maxLLMLogPayloadLength {
		return payload
	}
	return payload[:maxLLMLogPayloadLength] + "\n... [truncated]"
}
