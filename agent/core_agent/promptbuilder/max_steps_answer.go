package promptbuilder

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"text/template"
)

const MaxStepsAnswerOnlyPromptBuilderName = "max_steps_answer_only"

//go:embed templates/max_steps_answer.tmpl
var maxStepsAnswerTemplate string

var maxStepsAnswerTmpl *template.Template

func init() {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"marshalJSON": func(v interface{}) string {
			bytes, err := jsonMarshalIndentForTemplate(v)
			if err != nil {
				return fmt.Sprintf("%v", v)
			}
			return string(bytes)
		},
		"memoryContent": func(memory interface{ PromptContent() string }) string {
			if memory == nil {
				return ""
			}
			return memory.PromptContent()
		},
		"contextUsagePercent": func(info *ContextInfo) int {
			if info == nil || info.LimitTokens <= 0 || info.CurrentTokens <= 0 {
				return 0
			}
			percent := info.CurrentTokens * 100 / info.LimitTokens
			if percent < 0 {
				return 0
			}
			if percent > 100 {
				return 100
			}
			return percent
		},
	}
	var err error
	maxStepsAnswerTmpl, err = template.New("max_steps_answer").Funcs(funcMap).Parse(maxStepsAnswerTemplate)
	if err != nil {
		panic(fmt.Sprintf("failed to parse max_steps_answer template: %v", err))
	}
	Register(MaxStepsAnswerOnlyPromptBuilderName, NewMaxStepsAnswerOnlyPromptBuilder)
}

func jsonMarshalIndentForTemplate(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "   ", "  ")
}

// NewMaxStepsAnswerOnlyPromptBuilder builds the final-turn prompt when the main loop hits MaxSteps without answer.
func NewMaxStepsAnswerOnlyPromptBuilder(params map[string]interface{}) Builder {
	return func(input Input) (string, error) {
		tools := input.Tools
		if len(tools) == 0 {
			return "", fmt.Errorf("max_steps_answer_only requires at least one tool definition")
		}
		data := map[string]interface{}{
			"Memory":            input.Memory,
			"UserInput":         input.UserInput,
			"ContextInfo":       input.ContextInfo,
			"Tools":             tools,
			"NativeOpenAITools": input.NativeOpenAITools,
		}
		var buf bytes.Buffer
		if err := maxStepsAnswerTmpl.Execute(&buf, data); err != nil {
			return "", fmt.Errorf("execute max_steps_answer template: %w", err)
		}
		return buf.String(), nil
	}
}
