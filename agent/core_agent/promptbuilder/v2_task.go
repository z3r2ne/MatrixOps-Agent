package promptbuilder

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
)

const V2TaskPromptBuilderName = "v2_task"

//go:embed templates/v2_task.tmpl
var v2TaskTemplate string

var v2TaskTmpl *template.Template

func init() {
	var err error
	v2TaskTmpl, err = template.New("v2_task").Funcs(standardTemplateFuncMap()).Parse(v2TaskTemplate)
	if err != nil {
		panic(fmt.Sprintf("failed to parse v2_task template: %v", err))
	}
	Register(V2TaskPromptBuilderName, NewV2TaskPromptBuilder)
}

func NewV2TaskPromptBuilder(params map[string]interface{}) Builder {
	_ = params
	return func(input Input) (string, error) {
		data := map[string]interface{}{
			"Memory":            input.Memory,
			"Tools":             input.Tools,
			"UserInput":         input.UserInput,
			"ContextInfo":       input.ContextInfo,
			"NativeOpenAITools": input.NativeOpenAITools,
		}
		var buf bytes.Buffer
		if err := v2TaskTmpl.Execute(&buf, data); err != nil {
			return "", fmt.Errorf("execute template: %w", err)
		}
		return buf.String(), nil
	}
}
