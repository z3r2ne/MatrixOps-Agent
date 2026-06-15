package coreagent

import (
	"fmt"
	"strings"

	corepromptbuilder "matrixops.local/core_agent/promptbuilder"
	agentmemory "matrixops.local/memory"
)

const (
	DefaultPromptBuilderName            = corepromptbuilder.V2TaskPromptBuilderName
	V3TaskPromptBuilderName             = corepromptbuilder.V3TaskPromptBuilderName
	MemoryCompactionPromptBuilderName   = corepromptbuilder.MemoryCompactionPromptBuilderName
	MemoryCompactionSystemPrompt        = corepromptbuilder.MemoryCompactionSystemPrompt
	MaxStepsAnswerOnlyPromptBuilderName = corepromptbuilder.MaxStepsAnswerOnlyPromptBuilderName
	FrontendEngineerPromptBuilderName   = corepromptbuilder.FrontendEngineerPromptBuilderName
	LeaderPromptBuilderName             = corepromptbuilder.LeaderPromptBuilderName
)

type ContextInfo = corepromptbuilder.ContextInfo

type V2TaskPromptData struct {
	Memory      *agentmemory.Memory
	Tools       []ToolDefinition
	UserInput   string
	ContextInfo *ContextInfo
}

type MemoryCompactionPromptPayload struct {
	Instruction string
	UserInput   string
}

type PromptBuilderOptions struct {
	ContextInfoBuilder func(state *RunState) *ContextInfo
	MemoryResolver     func(value any) *agentmemory.Memory
	WorkerExtraPrompt  string
	Params             map[string]interface{}
}

func CreatePromptBuilder(name string, options PromptBuilderOptions) (PromptBuilder, error) {
	builder, err := corepromptbuilder.Create(name, options.Params)
	if err != nil {
		return nil, err
	}
	v3Builder, err := corepromptbuilder.Create(corepromptbuilder.V3TaskPromptBuilderName, options.Params)
	if err != nil {
		return nil, err
	}
	memoryResolver := options.MemoryResolver
	if memoryResolver == nil {
		memoryResolver = defaultMemoryResolver
	}
	return func(state *RunState) (string, error) {
		tools := state.Tools
		if len(tools) == 0 {
			mergeOpt := PromptToolMergeOptions{}
			if state.OmitAnswerInPromptMerge {
				mergeOpt.ExcludeAnswer = true
			}
			tools = MergePromptToolDefinitions(nil, mergeOpt)
		}
		memory := memoryResolver(state.Memory)
		if !promptBuilderNeedsTranscript(name) {
			memory = memoryWithoutTranscript(memory)
		}
		input := corepromptbuilder.Input{
			Memory:            memory,
			Tools:             toolDefinitionsToPromptBuilders(tools),
			UserInput:         state.UserInput,
			WorkerExtraPrompt: options.WorkerExtraPrompt,
			NativeOpenAITools: state.NativeOpenAIToolCalls,
		}
		if options.ContextInfoBuilder != nil {
			input.ContextInfo = options.ContextInfoBuilder(state)
		}
		if state.MaxStepsExhaustedFinalPass {
			finalBuilder, err := corepromptbuilder.Create(corepromptbuilder.MaxStepsAnswerOnlyPromptBuilderName, options.Params)
			if err != nil {
				return "", err
			}
			return finalBuilder(input)
		}
		if state.NativeOpenAIToolCalls {
			return v3Builder(input)
		}
		return builder(input)
	}, nil
}

func promptBuilderNeedsTranscript(name string) bool {
	switch name {
	case corepromptbuilder.MemoryCompactionPromptBuilderName:
		return true
	default:
		return false
	}
}

func MustCreatePromptBuilder(name string, options PromptBuilderOptions) PromptBuilder {
	builder, err := CreatePromptBuilder(name, options)
	if err != nil {
		panic(err)
	}
	return builder
}

func RenderV2TaskPrompt(data V2TaskPromptData) (string, error) {
	builder, err := corepromptbuilder.Create(corepromptbuilder.V2TaskPromptBuilderName, nil)
	if err != nil {
		return "", err
	}
	tools := data.Tools
	if len(tools) == 0 {
		tools = MergePromptToolDefinitions(nil, PromptToolMergeOptions{})
	}
	return builder(corepromptbuilder.Input{
		Memory:      data.Memory,
		Tools:       toolDefinitionsToPromptBuilders(tools),
		UserInput:   data.UserInput,
		ContextInfo: data.ContextInfo,
	})
}

func RenderMemoryCompactionPrompt(memory *agentmemory.Memory, contextInfo *ContextInfo, userInput string, workerExtraPrompt string) (string, error) {
	payload, err := RenderMemoryCompactionPromptPayload(memory, contextInfo, userInput, workerExtraPrompt)
	if err != nil {
		return "", err
	}
	switch {
	case payload.Instruction != "" && payload.UserInput != "":
		return payload.Instruction + "\n\n=== user ===\n" + payload.UserInput, nil
	case payload.Instruction != "":
		return payload.Instruction, nil
	default:
		return payload.UserInput, nil
	}
}

func RenderMemoryCompactionPromptPayload(memory *agentmemory.Memory, contextInfo *ContextInfo, userInput string, workerExtraPrompt string) (MemoryCompactionPromptPayload, error) {
	builder, err := corepromptbuilder.Create(corepromptbuilder.MemoryCompactionPromptBuilderName, nil)
	if err != nil {
		return MemoryCompactionPromptPayload{}, err
	}
	taskPrompt, err := builder(corepromptbuilder.Input{
		Memory:            memory,
		ContextInfo:       contextInfo,
		UserInput:         userInput,
		WorkerExtraPrompt: workerExtraPrompt,
	})
	if err != nil {
		return MemoryCompactionPromptPayload{}, err
	}
	transcript := ""
	if memory != nil {
		transcript = strings.TrimSpace(memory.PromptContent())
	}
	if transcript == "" {
		return MemoryCompactionPromptPayload{}, fmt.Errorf("memory compaction input is empty")
	}
	userMessage := transcript
	if trimmedTask := strings.TrimSpace(taskPrompt); trimmedTask != "" {
		userMessage = transcript + "\n\n" + trimmedTask
	}
	return MemoryCompactionPromptPayload{
		Instruction: MemoryCompactionSystemPrompt,
		UserInput:   userMessage,
	}, nil
}

func defaultMemoryResolver(value any) *agentmemory.Memory {
	switch typed := value.(type) {
	case *agentmemory.Memory:
		if typed == nil {
			return &agentmemory.Memory{}
		}
		return typed
	case agentmemory.Memory:
		copied := typed
		return &copied
	case interface{ PromptContent() string }:
		return &agentmemory.Memory{History: []*agentmemory.ChatHistoryItem{{Role: "assistant", Content: typed.PromptContent()}}}
	default:
		return &agentmemory.Memory{}
	}
}

func memoryWithoutTranscript(memory *agentmemory.Memory) *agentmemory.Memory {
	if memory == nil {
		return &agentmemory.Memory{}
	}
	cloned := *memory
	cloned.Entries = nil
	cloned.History = nil
	cloned.LatestToolCall = nil
	if len(memory.ProjectFilePrompt) > 0 {
		cloned.ProjectFilePrompt = append([]agentmemory.FilePrompt(nil), memory.ProjectFilePrompt...)
	}
	if len(memory.SkillPrompts) > 0 {
		cloned.SkillPrompts = append([]agentmemory.FilePrompt(nil), memory.SkillPrompts...)
	}
	return &cloned
}

func toolDefinitionsToPromptBuilders(defs []ToolDefinition) []corepromptbuilder.ToolDefinition {
	out := make([]corepromptbuilder.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		out = append(out, corepromptbuilder.ToolDefinition{Name: def.Name, Description: def.Description, Schema: def.Schema})
	}
	return out
}
