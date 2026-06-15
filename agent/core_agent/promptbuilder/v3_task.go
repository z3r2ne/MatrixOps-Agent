package promptbuilder

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
)

const V3TaskPromptBuilderName = "v3_task"

//go:embed templates/v3_task.tmpl
var v3TaskTemplate string

var v3TaskTmpl *template.Template

func init() {
	var err error
	v3TaskTmpl, err = template.New("v3_task").Funcs(standardTemplateFuncMap()).Parse(v3TaskTemplate)
	if err != nil {
		panic(fmt.Sprintf("failed to parse v3_task template: %v", err))
	}
	Register(V3TaskPromptBuilderName, NewV3TaskPromptBuilder)
}

func NewV3TaskPromptBuilder(params map[string]interface{}) Builder {
	return func(input Input) (string, error) {
		data := map[string]interface{}{
			"Memory": input.Memory,
		}
		var buf bytes.Buffer
		if err := v3TaskTmpl.Execute(&buf, data); err != nil {
			return "", fmt.Errorf("execute template: %w", err)
		}
		return buf.String(), nil
	}
}
