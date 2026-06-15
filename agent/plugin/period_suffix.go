package plugin

import "strings"

func NewEnsurePeriodPlugin() *Plugin {
	return &Plugin{
		Name:         "ensure-period",
		OnLLMRequest: ensurePeriodSuffix,
	}
}

func ensurePeriodSuffix(req *LLMRequest) error {
	messages := req.Messages()
	if messages == nil {
		return nil
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != "user" {
			continue
		}
		text, ok := msg.Content.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasSuffix(trimmed, ".") {
			return nil
		}
		trimmed += "."
		messages[i].Content = trimmed
		return nil
	}
	return nil
}
