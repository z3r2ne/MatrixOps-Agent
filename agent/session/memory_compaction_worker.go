package session

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"matrixops-agent/types"
	"pkgs/db/models"
	"pkgs/httpclient"

	coreagent "matrixops.local/core_agent"
	"matrixops.local/core_agent/workersv2/generic"

	"gorm.io/gorm"
)

type MemoryCompactionWorkerChatOptions struct {
	Context        context.Context
	DB             *gorm.DB
	HTTPClient     *http.Client
	Memory         *types.Memory
	UserInput      string
	ContextInfo    *coreagent.ContextInfo
	OnSummaryDelta func(string)
}

func RunMemoryCompactionWorkerChat(options MemoryCompactionWorkerChatOptions) (string, compactionPromptInfo, MemoryCompactionStreamResult, error) {
	if options.Context == nil {
		options.Context = context.Background()
	}
	if options.DB == nil {
		return "", compactionPromptInfo{}, MemoryCompactionStreamResult{}, fmt.Errorf("database is not configured")
	}
	if options.Memory == nil || len(options.Memory.Entries) == 0 {
		return "", compactionPromptInfo{}, MemoryCompactionStreamResult{}, fmt.Errorf("memory compaction input is empty")
	}
	userInput := strings.TrimSpace(options.UserInput)
	if userInput == "" {
		userInput = MemoryCompactionUserInstruction
	}

	compactionRuntime, err := ResolveMemoryCompactionRuntime(options.DB)
	if err != nil {
		return "", compactionPromptInfo{}, MemoryCompactionStreamResult{}, err
	}

	compactionMemory := SanitizeMemoryForCompactionPrompt(options.Memory)
	payload, err := coreagent.RenderMemoryCompactionPromptPayload(
		compactionMemory,
		options.ContextInfo,
		userInput,
		compactionRuntime.WorkerExtraPrompt(),
	)
	promptInfo := compactionPromptInfo{UserPrompt: payload.UserInput}
	if err != nil {
		return "", promptInfo, MemoryCompactionStreamResult{}, err
	}
	promptInfo.SystemPrompt = strings.TrimSpace(payload.Instruction)

	worker, err := newMemoryCompactionGenericWorker(options.DB, compactionRuntime, compactionMemory, options.ContextInfo)
	if err != nil {
		return "", promptInfo, MemoryCompactionStreamResult{}, err
	}

	httpClient := options.HTTPClient
	if httpClient == nil && compactionRuntime.LLMConfig != nil {
		httpClient = httpclient.ClientWithOptionalProxy(compactionRuntime.LLMConfig.Proxy)
	}

	onDelta := throttleSummaryDelta(options.OnSummaryDelta, 0)
	var lastDelta string
	chatResult, err := worker.Chat(options.Context, userInput, generic.ChatOptions{
		ExecuteOnce: true,
		MaxSteps:    1,
		HTTPClient:  httpClient,
		Callbacks: generic.ChatCallbacks{
			OnPartUpdated: func(part *coreagent.Part) {
				if part == nil || part.Type != coreagent.PartTypeText {
					return
				}
				text := strings.TrimSpace(part.Text)
				if text == "" || text == lastDelta {
					return
				}
				lastDelta = text
				if onDelta != nil {
					onDelta(text)
				}
			},
		},
	})
	if err != nil {
		return "", promptInfo, MemoryCompactionStreamResult{}, err
	}

	rawSummary := ""
	if chatResult != nil {
		rawSummary = strings.TrimSpace(chatResult.Answer)
	}
	if rawSummary == "" {
		return "", promptInfo, MemoryCompactionStreamResult{}, fmt.Errorf("memory compaction summary is empty")
	}

	summary, err := sanitizeMemoryCompactionSummary(rawSummary)
	if err != nil {
		return "", promptInfo, MemoryCompactionStreamResult{}, err
	}
	if onDelta != nil && summary != lastDelta {
		onDelta(summary)
	}

	streamResult := MemoryCompactionStreamResult{
		Summary: summary,
		Model:   compactionRuntime.ModelName(),
	}
	if chatResult != nil && chatResult.Result != nil && chatResult.Result.State != nil && chatResult.Result.State.Assistant != nil {
		streamResult.Finish = strings.TrimSpace(chatResult.Result.State.Assistant.Finish)
	}

	return summary, promptInfo, streamResult, nil
}

func newMemoryCompactionGenericWorker(db *gorm.DB, runtime *MemoryCompactionRuntime, memory *types.Memory, contextInfo *coreagent.ContextInfo) (*generic.Worker, error) {
	if runtime == nil {
		return nil, fmt.Errorf("memory compaction runtime is not configured")
	}

	opts := []generic.Option{
		generic.WithWorkerFromDB(db, models.WorkerCompaction),
		generic.WithPromptSections("", "", "", []generic.StaticPromptSection{
			{Name: "system_prompt", Content: coreagent.MemoryCompactionSystemPrompt},
		}, nil),
		generic.WithLoop(memoryCompactionPromptBuilder(memory, contextInfo, runtime.WorkerExtraPrompt()), "", coreagent.PromptBuilderOptions{}),
		generic.WithCompatibleActionSchemas([]coreagent.ActionSchema{coreagent.AnswerActionSchema}),
		generic.WithTools(coreagent.NewToolRegistry()),
		generic.WithMemory(generic.MemorySystem{
			// Compaction uses a single user message (transcript + task prompt), not per-entry history.
			Build: func(state *coreagent.RunState) (any, error) {
				return &types.Memory{}, nil
			},
		}),
		memoryCompactionProviderOptionsOption(runtime),
	}
	return generic.New(opts...)
}

func memoryCompactionPromptBuilder(memory *types.Memory, contextInfo *coreagent.ContextInfo, workerExtraPrompt string) coreagent.PromptBuilder {
	memorySnapshot := memory
	return func(state *coreagent.RunState) (string, error) {
		userInput := ""
		if state != nil {
			userInput = state.UserInput
		}
		payload, err := coreagent.RenderMemoryCompactionPromptPayload(
			memorySnapshot,
			contextInfo,
			userInput,
			workerExtraPrompt,
		)
		if err != nil {
			return "", err
		}
		return capCompactionPromptTranscript(payload.UserInput), nil
	}
}

func memoryCompactionProviderOptionsOption(runtime *MemoryCompactionRuntime) generic.Option {
	return func(c *generic.AgentConfig) error {
		if runtime != nil {
			c.Runtime.SystemPromptPlacement = runtime.SystemPromptPlacement()
		}
		if runtime != nil && runtime.LLMConfig != nil {
			cloned := *runtime.LLMConfig
			c.LLM.ProviderOptions = &cloned
		}
		if runtime != nil && runtime.Worker != nil {
			c.LLM.Model = strings.TrimSpace(runtime.Worker.Model)
			if runtime.Worker.Temperature != nil {
				c.LLM.Temperature = *runtime.Worker.Temperature
			}
			c.LLM.TopP = runtime.Worker.TopP
		}
		_, _, _, maxOut := memoryCompactionModelRequest(runtime)
		if maxOut > 0 {
			c.LLM.MaxOutputTokens = maxOut
		}
		return nil
	}
}
