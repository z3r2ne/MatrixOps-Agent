package coreagent

import "strings"

const (
	SystemPromptPlacementSystem      = "system"
	SystemPromptPlacementInstruction = "instruction"
	SystemPromptPlacementUserInput   = "user_input"
)

type PromptPayload struct {
	UserPrompt   string
	SystemPrompt string
	Instruction  string
}

func NormalizeSystemPromptPlacement(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case SystemPromptPlacementSystem:
		return SystemPromptPlacementSystem
	case SystemPromptPlacementInstruction:
		return SystemPromptPlacementInstruction
	case SystemPromptPlacementUserInput:
		return SystemPromptPlacementUserInput
	default:
		return SystemPromptPlacementUserInput
	}
}

func PreparePromptPayload(prompt string, placement string) PromptPayload {
	fullPrompt := strings.TrimSpace(prompt)
	switch NormalizeSystemPromptPlacement(placement) {
	case SystemPromptPlacementSystem:
		systemPrompt, userPrompt, ok := extractWrappedPromptSection(fullPrompt, "system_prompt")
		if !ok {
			return PromptPayload{UserPrompt: fullPrompt}
		}
		return PromptPayload{
			UserPrompt:   userPrompt,
			SystemPrompt: systemPrompt,
		}
	case SystemPromptPlacementInstruction:
		systemPrompt, userPrompt, ok := extractWrappedPromptSection(fullPrompt, "system_prompt")
		if !ok {
			return PromptPayload{UserPrompt: fullPrompt}
		}
		return PromptPayload{
			UserPrompt:  userPrompt,
			Instruction: systemPrompt,
		}
	default:
		return PromptPayload{UserPrompt: fullPrompt}
	}
}

func PrepareFullPromptPayload(prompt string, placement string, userInput string) PromptPayload {
	fullPrompt := strings.TrimSpace(prompt)
	userInput = strings.TrimSpace(userInput)
	switch NormalizeSystemPromptPlacement(placement) {
	case SystemPromptPlacementSystem:
		return PromptPayload{
			UserPrompt:   userInput,
			SystemPrompt: fullPrompt,
		}
	case SystemPromptPlacementInstruction:
		return PromptPayload{
			UserPrompt:  userInput,
			Instruction: fullPrompt,
		}
	default:
		return PromptPayload{
			UserPrompt: joinPromptParts(fullPrompt, userInput),
		}
	}
}

func joinPromptParts(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return strings.Join(out, "\n\n")
}

func extractWrappedPromptSection(prompt string, tag string) (section string, remainder string, ok bool) {
	prompt = strings.TrimSpace(prompt)
	tag = strings.TrimSpace(tag)
	if prompt == "" || tag == "" {
		return "", prompt, false
	}

	startTag := "<" + tag + ">"
	endTag := "</" + tag + ">"

	start := strings.Index(prompt, startTag)
	if start < 0 {
		return "", prompt, false
	}
	endRel := strings.Index(prompt[start:], endTag)
	if endRel < 0 {
		return "", prompt, false
	}
	end := start + endRel + len(endTag)

	section = strings.TrimSpace(prompt[start:end])
	before := strings.TrimSpace(prompt[:start])
	after := strings.TrimSpace(prompt[end:])
	switch {
	case before != "" && after != "":
		remainder = before + "\n\n" + after
	case before != "":
		remainder = before
	default:
		remainder = after
	}
	return section, strings.TrimSpace(remainder), true
}
