package session

import (
	"fmt"

	sessionmemory "matrixops-agent/session/memory"
	"matrixops-agent/types"
	"pkgs/db/storage"
)

type ProcessV2Hooks struct {
	PrepareMemory       func(r *AgentRunner, runtimeConfig *RuntimeConfig) error
	BuildMemory         func(r *AgentRunner, runtimeConfig *RuntimeConfig) (*types.Memory, error)
	PersistTokens       func(r *AgentRunner, runtimeConfig *RuntimeConfig, tokens *MessageTokens) error
	LogPromptError      func(r *AgentRunner, command string, errorText string, rawRequest string, rawResponse string)
	CaptureFinishResult func(r *AgentRunner, runtimeConfig *RuntimeConfig) error
}

type ProcessV2MemoryState = sessionmemory.ProcessV2MemoryState

func NewProcessV2MemoryState(base *types.Memory) *ProcessV2MemoryState {
	return sessionmemory.NewProcessV2MemoryState(base)
}

func (r *AgentRunner) ensureRuntimeMemoryState(runtimeConfig *RuntimeConfig) error {
	if runtimeConfig == nil {
		return nil
	}
	if runtimeConfig.MemoryState != nil {
		return nil
	}

	base := cloneProcessV2Memory(runtimeConfig.BaseMemory)
	if base == nil && r.db != nil && r.GetSessionID() != "" && runtimeConfig.Worker != nil {
		taskForMemory := r.task
		if taskForMemory == nil {
			return fmt.Errorf("task is required to initialize runtime memory")
		}
		loaded, err := r.getMemory(taskForMemory, r.db, r.GetSessionID(), runtimeConfig.Worker.Name)
		if err != nil {
			return fmt.Errorf("load runtime memory: %w", err)
		}
		base = loaded
	}
	if base == nil {
		base = &types.Memory{}
	}
	runtimeConfig.MemoryState = NewProcessV2MemoryState(base)
	if runtimeConfig.BaseMemory == nil {
		runtimeConfig.BaseMemory = cloneProcessV2Memory(base)
	}
	return nil
}

func (r *AgentRunner) prepareProcessV2Memory(runtimeConfig *RuntimeConfig) error {
	if err := r.ensureRuntimeMemoryState(runtimeConfig); err != nil {
		return fmt.Errorf("ensure runtime memory: %w", err)
	}
	if runtimeConfig != nil && runtimeConfig.ProcessV2Hooks != nil && runtimeConfig.ProcessV2Hooks.PrepareMemory != nil {
		return runtimeConfig.ProcessV2Hooks.PrepareMemory(r, runtimeConfig)
	}
	if runtimeConfig != nil && runtimeConfig.ManualMemoryCompactionRequested {
		runtimeConfig.ManualMemoryCompactionRequested = false
		return r.forceOrganizeProcessV2MemoryNow(runtimeConfig)
	}
	return r.forceOrganizeProcessV2MemoryIfNeeded(runtimeConfig)
}

func (r *AgentRunner) buildProcessV2Memory(runtimeConfig *RuntimeConfig) (*types.Memory, error) {
	if runtimeConfig != nil && runtimeConfig.ProcessV2Hooks != nil && runtimeConfig.ProcessV2Hooks.BuildMemory != nil {
		return runtimeConfig.ProcessV2Hooks.BuildMemory(r, runtimeConfig)
	}
	if runtimeConfig != nil && runtimeConfig.MemoryState != nil {
		return runtimeConfig.MemoryState.Snapshot(), nil
	}
	if runtimeConfig != nil && runtimeConfig.BaseMemory != nil {
		return cloneProcessV2Memory(runtimeConfig.BaseMemory), nil
	}
	if r.db == nil {
		return &types.Memory{}, nil
	}
	return r.getMemory(r.task, r.db, r.GetSessionID(), runtimeConfig.Worker.Name)
}

func (r *AgentRunner) persistProcessV2Tokens(runtimeConfig *RuntimeConfig, tokens *MessageTokens) error {
	if tokens == nil {
		return nil
	}
	if runtimeConfig != nil {
		runtimeConfig.SessionTokens = tokens
	}
	if r.session != nil {
		r.session.Tokens = tokens
	}
	if runtimeConfig != nil && runtimeConfig.ProcessV2Hooks != nil && runtimeConfig.ProcessV2Hooks.PersistTokens != nil {
		return runtimeConfig.ProcessV2Hooks.PersistTokens(r, runtimeConfig, tokens)
	}
	if r.db == nil {
		return nil
	}
	return storage.UpdateSessionTokens(r.db, r.GetSessionID(), tokens)
}

func (r *AgentRunner) captureProcessV2Finish(runtimeConfig *RuntimeConfig) error {
	if runtimeConfig != nil && runtimeConfig.ProcessV2Hooks != nil && runtimeConfig.ProcessV2Hooks.CaptureFinishResult != nil {
		return runtimeConfig.ProcessV2Hooks.CaptureFinishResult(r, runtimeConfig)
	}
	if runtimeConfig == nil || runtimeConfig.Assistant == nil || runtimeConfig.Assistant.Snapshot == "" {
		return nil
	}
	return r.captureFinishSnapshotV2(runtimeConfig)
}

func (r *AgentRunner) logProcessV2PromptError(runtimeConfig *RuntimeConfig, command string, errorText string, rawRequest string, rawResponse string) {
	if runtimeConfig != nil && runtimeConfig.ProcessV2Hooks != nil && runtimeConfig.ProcessV2Hooks.LogPromptError != nil {
		runtimeConfig.ProcessV2Hooks.LogPromptError(r, command, errorText, rawRequest, rawResponse)
		return
	}
	r.logV2PromptError(command, errorText, rawRequest, rawResponse)
}

func cloneProcessV2Memory(memory *types.Memory) *types.Memory {
	return sessionmemory.CloneProcessV2Memory(memory)
}
