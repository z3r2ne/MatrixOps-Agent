package session

import (
	"context"
	"fmt"
	"time"

	"matrixops-agent/llm"
	"matrixops-agent/tool"
	"matrixops-agent/types"
	"matrixops-agent/util"
	database "pkgs/db"
	"pkgs/db/models"

	coreagent "matrixops.local/core_agent"
)

type ProcessV2SimpleInput struct {
	Context                       context.Context
	Client                        llm.ChatClient
	Model                         string
	SystemPrompt                  string
	UserInput                     string
	Tools                         *tool.Registry
	History                       []*types.ChatHistoryItem
	BaseMemory                    *types.Memory
	ProviderOptions               *models.LLMConfig
	ActionSchemas                 []coreagent.ActionSchema
	OnEvent                       func(name string, payload interface{})
	Hooks                         *ProcessV2Hooks
	// ExecuteOnce runs a single model iteration (see core_agent.RunState.SingleStep).
	ExecuteOnce bool
	MaxSteps    int
}

type ProcessV2SimpleResult struct {
	SessionID string
	MessageID string
	Answer    string
	Memory    *types.Memory
	Parts     []*Part
	Assistant *MessageInfo
}

func RunProcessV2Simple(input ProcessV2SimpleInput) (*ProcessV2SimpleResult, error) {
	if input.Client == nil {
		return nil, fmt.Errorf("client is required")
	}
	if input.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	now := time.Now().UnixMilli()
	sessionInfo := &types.Info{
		ID:        util.Descending("session"),
		Slug:      util.Slug(),
		ProjectID: "simple-v2",
		Directory: ".",
		Title:     createDefaultTitle(false),
		Version:   util.Version(),
		Time: types.TimeInfo{
			Created: now,
			Updated: now,
		},
	}

	emitter := NewEmitter(nil, sessionInfo.ID)
	if input.OnEvent != nil {
		bindProcessV2SimpleEvents(emitter, input.OnEvent)
	}

	worker := &models.Worker{
		Name:        "simple-v2",
		Model:       input.Model,
		Temperature: nil,
		TopP:        0,
	}

	baseMemory := input.BaseMemory
	if baseMemory == nil {
		baseMemory = &types.Memory{
			GlobalPrompt: input.SystemPrompt,
			History:      cloneProcessV2History(input.History),
		}
	}
	baseMemory.GlobalPrompt = appendTaskLoopGuidance(baseMemory.GlobalPrompt)

	runner, err := NewAgentRunner(
		WithCtx(input.Context),
		WithLLM(input.Client),
		WithEmitter(emitter),
		WithSession(sessionInfo),
		WithProjectID(sessionInfo.ProjectID),
		WithDirectory(sessionInfo.Directory),
		WithWorkerModel(worker),
		WithLLMConfigModel(input.ProviderOptions),
		WithTools(input.Tools),
		WithBaseMemory(baseMemory),
		WithProcessV2Hooks(input.Hooks),
		WithActionSchemas(input.ActionSchemas),
	)
	if err != nil {
		return nil, err
	}

	runtimeConfig, err := runner.buildRuntimeConfig(NewAgentRunnerConfig(
		WithCtx(input.Context),
		WithLLM(input.Client),
		WithEmitter(emitter),
		WithSession(sessionInfo),
		WithProjectID(sessionInfo.ProjectID),
		WithDirectory(sessionInfo.Directory),
		WithWorkerModel(worker),
		WithLLMConfigModel(input.ProviderOptions),
		WithTools(input.Tools),
		WithBaseMemory(baseMemory),
		WithProcessV2Hooks(input.Hooks),
		WithActionSchemas(input.ActionSchemas),
		WithInputText(input.UserInput),
	))
	if err != nil {
		return nil, err
	}

	maxSteps := input.MaxSteps
	if maxSteps <= 0 {
		maxSteps = database.GetAgentMaxSteps(runner.db)
	}

	if err := runner.runTaskV2(runtimeConfig, ProcessInputV2{
		MaxSteps:    maxSteps,
		ExecuteOnce: input.ExecuteOnce,
		UserPrompt:  input.UserInput,
	}); err != nil {
		return nil, err
	}

	result := &ProcessV2SimpleResult{
		SessionID: sessionInfo.ID,
		Memory:    runtimeConfig.MemoryState.Snapshot(),
		Assistant: runtimeConfig.Assistant,
		MessageID: runtimeConfig.Assistant.ID,
	}

	if result.Memory != nil {
		for i := len(result.Memory.History) - 1; i >= 0; i-- {
			item := result.Memory.History[i]
			if item != nil && item.Role == "assistant" && item.Content != "" {
				result.Answer = item.Content
				break
			}
		}
	}

	return result, nil
}

func bindProcessV2SimpleEvents(emitter *Emitter, onEvent func(name string, payload interface{})) {
	eventNames := []string{
		EventMessageUpdated,
		EventPartUpdated,
		EventSessionError,
		EventPluginVarSet,
		EventWaitUserInput,
	}
	for _, eventName := range eventNames {
		name := eventName
		emitter.On(name, func(args ...interface{}) {
			if len(args) == 0 {
				onEvent(name, nil)
				return
			}
			onEvent(name, args[0])
		})
	}
}

func cloneProcessV2History(history []*types.ChatHistoryItem) []*types.ChatHistoryItem {
	if len(history) == 0 {
		return nil
	}
	out := make([]*types.ChatHistoryItem, 0, len(history))
	for _, item := range history {
		if item == nil {
			continue
		}
		copied := *item
		out = append(out, &copied)
	}
	return out
}
