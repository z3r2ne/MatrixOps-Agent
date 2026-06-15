package promptbuilder

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
)

const LeaderPromptBuilderName = "leader"

//go:embed templates/leader.tmpl
var leaderTemplate string

var leaderTmpl *template.Template

func init() {
	var err error
	leaderTmpl, err = template.New("leader").Funcs(standardTemplateFuncMap()).Parse(leaderTemplate)
	if err != nil {
		panic(fmt.Sprintf("failed to parse leader template: %v", err))
	}
	Register(LeaderPromptBuilderName, NewLeaderPromptBuilder)
}

// NewLeaderPromptBuilder 与 v2_task 相同参数语义。
func NewLeaderPromptBuilder(params map[string]interface{}) Builder {
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
		if err := leaderTmpl.Execute(&buf, data); err != nil {
			return "", fmt.Errorf("execute template: %w", err)
		}
		return buf.String(), nil
	}
}
