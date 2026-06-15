package coreagent

import (
	"strings"
)

func resolveToolResultBodyAndSystem(result ToolResult, err error) (systemMessage string, body string, isError bool) {
	isError = err != nil || result.IsError
	body = result.Content
	systemMessage = strings.TrimSpace(result.Message)

	if isError {
		if systemMessage == "" && err != nil {
			systemMessage = err.Error()
		}
		return systemMessage, body, true
	}

	if strings.TrimSpace(body) == "" && systemMessage == "" {
		systemMessage = emptyToolSystemMessage
	}
	return systemMessage, body, false
}
