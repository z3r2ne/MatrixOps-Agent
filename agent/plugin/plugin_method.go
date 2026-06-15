package plugin

func NewStreamEventPlugin(callback StreamEventHook) *Plugin {
	return &Plugin{
		Name:          "stream-event",
		OnStreamEvent: callback,
	}
}

func NewRequestEventPlugin(callback LLMRequestHook) *Plugin {
	return &Plugin{
		Name:         "llm-request",
		OnLLMRequest: callback,
	}
}
