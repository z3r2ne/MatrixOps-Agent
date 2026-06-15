package session

import (
	"fmt"
	"time"
)

// Prompt 是对外暴露的主入口函数
// 创建 AgentRunner 实例并执行 Prompt 操作
func Prompt(options ...AgentRunnerOption) (WithParts, error) {
	// 1. 构建 AgentRunnerConfig
	cfg := NewAgentRunnerConfig(options...)

	// 2. 验证配置
	if err := assertPromptConfig(cfg); err != nil {
		return WithParts{}, fmt.Errorf("prompt: invalid config: %w", err)
	}

	// 3. 创建 AgentRunner
	runner, err := NewAgentRunner(options...)
	if err != nil {
		return WithParts{}, fmt.Errorf("prompt: new agent runner: %w", err)
	}

	// 4. 构建 RuntimeConfig
	runtimeConfig, err := runner.buildRuntimeConfig(cfg)
	if err != nil {
		return WithParts{}, fmt.Errorf("prompt: build runtime config: %w", err)
	}

	// 5. 执行 Prompt
	return runner.Prompt(runtimeConfig)
}

// Prompt 执行对话的主入口方法
func (r *AgentRunner) Prompt(runtimeConfig *RuntimeConfig) (WithParts, error) {
	var userText string
	var err error
	if !runtimeConfig.SkipCreateUserMessage {
		userText, _, err = r.createUserMessage(runtimeConfig)
		if err != nil {
			return WithParts{}, fmt.Errorf("create user message: %w", err)
		}
	}

	// 如果是新会话，异步确保标题
	if r.isNewSession {
		r.RunAsyncAITask(func() error {
			return r.ensureTitle(runtimeConfig)
		})
	}

	if runtimeConfig.ManualMemoryCompactionRequested {
		if _, err := r.emitter.UpdateMessage(runtimeConfig.Assistant); err != nil {
			return WithParts{}, fmt.Errorf("update assistant message: %w", err)
		}
		err := r.forceOrganizeProcessV2MemoryNow(runtimeConfig)
		runtimeConfig.Assistant.State = "completed"
		runtimeConfig.Assistant.Time.Completed = time.Now().UnixMilli()
		_, _ = r.emitter.UpdateMessage(runtimeConfig.Assistant)
		if err != nil {
			return WithParts{}, fmt.Errorf("manual memory compaction: %w", err)
		}
		return WithParts{}, nil
	}
	if runtimeConfig.ManualSessionSummaryRequested {
		if _, err := r.emitter.UpdateMessage(runtimeConfig.Assistant); err != nil {
			return WithParts{}, fmt.Errorf("update assistant message: %w", err)
		}
		err := r.runManualSessionSummary(runtimeConfig)
		runtimeConfig.Assistant.State = "completed"
		runtimeConfig.Assistant.Time.Completed = time.Now().UnixMilli()
		_, _ = r.emitter.UpdateMessage(runtimeConfig.Assistant)
		if err != nil {
			return WithParts{}, fmt.Errorf("manual session summary: %w", err)
		}
		return WithParts{}, nil
	}

	if runtimeConfig.NewWorktreeBranch != "" {
		if err := r.handleNewWorktreeCommand(runtimeConfig); err != nil {
			r.emitCommandFailure(runtimeConfig, "new-worktree", err)
			return WithParts{}, fmt.Errorf("new-worktree: %w", err)
		}
		return WithParts{}, nil
	}

	// 检查是否有 worker 提及，如果有则分发给对应的 worker
	workers := extractWorkerMentions(runtimeConfig.Parts)
	if len(workers) == 0 {
		return r._prompt(runtimeConfig)
	}

	// 有 worker 提及，分发给相应的 worker 处理
	var result WithParts
	for _, workerName := range workers {
		workerCfg := runtimeConfig.clone()
		err := workerCfg.SetWorker(r.db, workerName)
		if err != nil {
			return WithParts{}, fmt.Errorf("get worker by name: %w", err)
		}
		workerCfg.ForceContinue = true
		workerCfg.SetUserInput(userText)
		res, err := r._prompt(workerCfg)
		if err != nil {
			return res, err
		}
		result = res
	}
	return result, nil
}

// _prompt 内部的 Prompt 实现，处理单个 worker 的对话
func (r *AgentRunner) _prompt(runtimeConfig *RuntimeConfig) (WithParts, error) {
	if _, err := r.GetSessionInfo(); err != nil {
		return WithParts{}, fmt.Errorf("prompt: session not found (session_id=%s): %w", r.GetSessionID(), err)
	}

	err := r.runTaskV2(runtimeConfig)
	if err != nil {
		return WithParts{}, fmt.Errorf("run task v2: %w", err)
	}
	return WithParts{}, nil
}

// RunAsyncAITask 异步执行 AI 任务
func (r *AgentRunner) RunAsyncAITask(task func() error) error {
	r.wg.Add()
	go func() {
		defer r.wg.Done()
		err := task()
		if err != nil {
			r.emitter.Emit(EventError, err)
		}
	}()
	return nil
}

// GetSessionInfo 获取会话信息
func (r *AgentRunner) GetSessionInfo() (*Info, error) {
	if r.session != nil {
		return r.session, nil
	}
	if r.db == nil {
		return nil, fmt.Errorf("session info unavailable without db")
	}
	sessionInfo, err := getSession(r.db, r.GetSessionID())
	if err != nil {
		return nil, fmt.Errorf("get session info (session_id=%s): %w", r.GetSessionID(), err)
	}
	return sessionInfo, nil
}
