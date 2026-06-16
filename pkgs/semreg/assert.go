package semreg

import (
	"fmt"
	"strings"
)

// AssertResult 结构断言结果。
type AssertResult struct {
	Passed  bool
	Errors  []string
	Details map[string]string
}

// EvaluateStructAssertions 对 system prompt 与 user input 执行 L0 断言。
func EvaluateStructAssertions(systemPrompt, userInput string, spec AssertSpec) AssertResult {
	result := AssertResult{
		Passed:  true,
		Details: map[string]string{},
	}
	for _, fragment := range spec.SystemPromptContains {
		if !strings.Contains(systemPrompt, fragment) {
			result.Passed = false
			result.Errors = append(result.Errors, fmt.Sprintf("system prompt 缺少片段 %q", fragment))
		}
	}
	for _, fragment := range spec.SystemPromptNotContains {
		if strings.Contains(systemPrompt, fragment) {
			result.Passed = false
			result.Errors = append(result.Errors, fmt.Sprintf("system prompt 不应包含 %q", fragment))
		}
	}
	if want := strings.TrimSpace(spec.UserInputEquals); want != "" {
		if strings.TrimSpace(userInput) != want {
			result.Passed = false
			result.Errors = append(result.Errors, fmt.Sprintf("user input = %q, want %q", userInput, want))
		}
	}
	result.Details["system_prompt_len"] = fmt.Sprintf("%d", len(systemPrompt))
	return result
}

// MergeAssertResults 合并多个断言结果。
func MergeAssertResults(results ...AssertResult) AssertResult {
	merged := AssertResult{
		Passed:  true,
		Details: map[string]string{},
	}
	for _, result := range results {
		if !result.Passed {
			merged.Passed = false
		}
		merged.Errors = append(merged.Errors, result.Errors...)
		for key, value := range result.Details {
			merged.Details[key] = value
		}
	}
	return merged
}
