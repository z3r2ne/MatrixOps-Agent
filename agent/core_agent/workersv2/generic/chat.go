package generic

import (
	"context"
	"fmt"
	"net/http"
	"time"

	coreagent "matrixops.local/core_agent"
)

// ChatCallbacks 可选的中间过程回调（工具流式更新、自定义事件等通过 Emitter 转发）。
type ChatCallbacks struct {
	OnMessageUpdated func(*coreagent.Message)
	OnPartUpdated    func(*coreagent.Part)
	OnEmit           func(name string, payload any)
}

// ChatOptions 单次 Chat 的选项。
type ChatOptions struct {
	SessionID     string
	Assistant     *coreagent.Message
	MaxSteps      int
	ExecuteOnce   bool
	Tools         []coreagent.ToolDefinition
	HTTPClient    *http.Client
	OnRawRequest  func(raw string)
	OnRawResponse func(raw string)
	OnRetryError  func(err error, retryAttempt int, maxRetries int, nextDelay time.Duration, attemptDuration time.Duration, rawResponse string)
	Callbacks     ChatCallbacks
}

// ChatResult Chat 完成后的结果。
type ChatResult struct {
	Answer string
	Result *RunResult
}

type callbackEmitter struct {
	inner coreagent.Emitter
	cbs   ChatCallbacks
}

func (e *callbackEmitter) UpdateMessage(info *coreagent.Message) (*coreagent.Message, error) {
	updated, err := e.inner.UpdateMessage(info)
	if e.cbs.OnMessageUpdated != nil {
		if updated != nil {
			e.cbs.OnMessageUpdated(updated)
		} else if info != nil {
			e.cbs.OnMessageUpdated(info)
		}
	}
	return updated, err
}

func (e *callbackEmitter) UpdatePart(part *coreagent.Part) (*coreagent.Part, error) {
	updated, err := e.inner.UpdatePart(part)
	if e.cbs.OnPartUpdated != nil {
		if updated != nil {
			e.cbs.OnPartUpdated(updated)
		} else if part != nil {
			e.cbs.OnPartUpdated(part)
		}
	}
	return updated, err
}

func (e *callbackEmitter) Emit(name string, payload interface{}) {
	e.inner.Emit(name, payload)
	if e.cbs.OnEmit != nil {
		e.cbs.OnEmit(name, payload)
	}
}

func chainEmitter(base coreagent.Emitter, cbs ChatCallbacks) coreagent.Emitter {
	if cbs.OnMessageUpdated == nil && cbs.OnPartUpdated == nil && cbs.OnEmit == nil {
		return base
	}
	if base == nil {
		base = coreagent.NoEmitter{}
	}
	return &callbackEmitter{inner: base, cbs: cbs}
}

// Chat 执行一轮用户输入到 answer：封装 Run，并可选注入 ChatCallbacks。
func (w *Worker) Chat(ctx context.Context, userInput string, opt ChatOptions) (*ChatResult, error) {
	if w == nil {
		return nil, fmt.Errorf("worker is nil")
	}

	runInput := RunInput{
		Context:        ctx,
		SessionID:      opt.SessionID,
		Assistant:      opt.Assistant,
		UserInput:      userInput,
		Tools:          opt.Tools,
		MaxSteps:       opt.MaxSteps,
		HTTPClient:     opt.HTTPClient,
		OnRawRequest:   opt.OnRawRequest,
		OnRawResponse:  opt.OnRawResponse,
		OnRetryError:   opt.OnRetryError,
		ExecuteOnce:    opt.ExecuteOnce,
		Callbacks:      opt.Callbacks,
	}

	w.resultMu.Lock()
	w.lastResult = nil
	w.lastErr = nil
	w.resultMu.Unlock()

	if err := w.SendMessage(runInput); err != nil {
		return nil, err
	}

	if err := w.Start(ctx); err != nil {
		return nil, err
	}

	w.resultMu.Lock()
	result := w.lastResult
	err := w.lastErr
	w.resultMu.Unlock()

	answer := ""
	if result != nil {
		answer = result.OutputText
	}
	return &ChatResult{
		Answer: answer,
		Result: result,
	}, err
}

// Ext 返回构造 Worker 时的扩展 JSON（如来自 DB）。
func (w *Worker) Ext() ExtConfig {
	if w == nil {
		return ExtConfig{}
	}
	return w.cfg.ext
}
