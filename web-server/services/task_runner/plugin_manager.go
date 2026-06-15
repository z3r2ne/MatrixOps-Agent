package task_runner

import (
	"encoding/json"
	"log"
	"strings"

	"matrixops-agent/llm"
	agentplugin "matrixops-agent/plugin"
)

const (
	pluginActuallyReq  = "actually-req"
	pluginEnsurePeriod = "ensure-period"
)

func buildAdditionalContent(request *agentplugin.LLMRequest) string {
	payload, err := json.Marshal(request)
	if err != nil {
		return ""
	}
	return string(payload)
}

func buildPromptPluginManager(taskID uint, cfg map[string]interface{}, rawInputSetter func(*agentplugin.LLMRequest)) *agentplugin.Manager {
	manager := agentplugin.NewManager()
	// if capture := newRawInputCapturePlugin(rawInputSetter); capture != nil {
	// 	manager.Register(capture)
	// }
	manager.Register(agentplugin.NewStreamEventPlugin(func(event *llm.StreamEvent) error {
		log.Println("stream event", "event", event)
		return nil
	}))

	// names := pluginNamesFromConfig(cfg)
	// // if len(names) == 0 {
	// // 	for _, plugin := range defaultPromptPlugins(taskID) {
	// // 		manager.Register(plugin)
	// // 	}
	// // 	return manager
	// // }

	// for _, name := range names {
	// 	if plugin := pluginFromName(taskID, name); plugin != nil {
	// 		manager.Register(plugin)
	// 	}
	// }

	return manager
}

func pluginNamesFromConfig(cfg map[string]interface{}) []string {
	if cfg == nil {
		return nil
	}
	raw, ok := cfg["plugins"]
	if !ok {
		return nil
	}

	seen := map[string]bool{}
	names := []string{}
	addName := func(name string) {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || seen[trimmed] {
			return
		}
		seen[trimmed] = true
		names = append(names, trimmed)
	}

	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			addName(toString(item))
		}
	case []string:
		for _, item := range v {
			addName(item)
		}
	case string:
		for _, item := range strings.Split(v, ",") {
			addName(item)
		}
	}

	return names
}

// func defaultPromptPlugins(taskID uint) []*agentplugin.Plugin {
// 	return []*agentplugin.Plugin{
// 		agentplugins.NewActuallyReqPlugin(func(request *agentplugin.LLMRequest) error {
// 			services.GetGlobalWSHub().BroadcastTaskStatus(taskID, "failed", "")
// 			return nil
// 		}),
// 	}
// }

// func pluginFromName(taskID uint, name string) *agentplugin.Plugin {
// 	switch strings.ToLower(strings.TrimSpace(name)) {
// 	case pluginActuallyReq:
// 		return agentplugins.NewActuallyReqPlugin(func(request *agentplugin.LLMRequest) error {
// 			services.GetGlobalWSHub().BroadcastTaskStatus(taskID, "failed", "")
// 			return nil
// 		})
// 	case pluginEnsurePeriod:
// 		return agentplugin.NewEnsurePeriodPlugin()
// 	default:
// 		return nil
// 	}
// }

func newRawInputCapturePlugin(setter func(*agentplugin.LLMRequest)) *agentplugin.Plugin {
	if setter == nil {
		return nil
	}
	return &agentplugin.Plugin{
		Name: "capture-raw-input",
		OnLLMRequest: func(request *agentplugin.LLMRequest) error {
			setter(request)
			// rawInput := extractRawUserInput(request)
			// if rawInput != "" {

			// }
			return nil
		},
	}
}

func extractRawUserInput(request *agentplugin.LLMRequest) string {
	if request == nil {
		return ""
	}
	messages := request.Messages()
	if messages == nil {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != "user" {
			continue
		}
		if text, ok := msg.Content.(string); ok {
			return text
		}
	}
	return ""
}
