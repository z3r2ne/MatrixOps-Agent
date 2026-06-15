package session

import (
	"errors"
)

func assertPromptConfig(cfg *AgentRunnerConfig) error {
	if cfg == nil {
		return errors.New("agent runner: missing config")
	}
	// if cfg.Task == nil {
	// 	return errors.New("agent runner: missing task")
	// }
	if cfg.Emitter == nil {
		return errors.New("agent runner: missing session bus")
	}
	if cfg.newTaskHandler == nil {
		return errors.New("agent runner: missing new task handler")
	}
	if cfg.TaskID == 0 {
		return errors.New("agent runner: missing task id")
	}
	if cfg.ProjectID == "" {
		return errors.New("agent runner: missing project id")
	}
	if cfg.Directory == "" {
		return errors.New("agent runner: missing directory")
	}
	if cfg.LLM == nil {
		return errors.New("agent runner: missing llm")
	}
	return nil
}
