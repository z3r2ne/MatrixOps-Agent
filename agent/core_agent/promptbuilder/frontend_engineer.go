package promptbuilder

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
)

const FrontendEngineerPromptBuilderName = "frontend_engineer"

//go:embed templates/frontend_engineer.tmpl
var frontendEngineerTemplate string

var frontendEngineerTmpl *template.Template

func init() {
	var err error
	frontendEngineerTmpl, err = template.New("frontend_engineer").Funcs(standardTemplateFuncMap()).Parse(frontendEngineerTemplate)
	if err != nil {
		panic(fmt.Sprintf("failed to parse frontend_engineer template: %v", err))
	}
	Register(FrontendEngineerPromptBuilderName, NewFrontendEngineerPromptBuilder)
}

// NewFrontendEngineerPromptBuilder 与 v2_task 相同参数语义。
func NewFrontendEngineerPromptBuilder(params map[string]interface{}) Builder {
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
		if err := frontendEngineerTmpl.Execute(&buf, data); err != nil {
			return "", fmt.Errorf("execute template: %w", err)
		}
		return buf.String(), nil
	}
}
