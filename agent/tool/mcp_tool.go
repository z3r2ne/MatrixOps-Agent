package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcppkg "pkgs/mcp"
)

type mcpTool struct {
	descriptor mcppkg.ToolDescriptor
}

func RegisterMcpTools(registry *Registry, manager *mcppkg.Manager) {
	if registry == nil || manager == nil {
		return
	}
	for _, descriptor := range manager.ToolDescriptors() {
		registry.Register(&mcpTool{descriptor: descriptor})
	}
}

func (t *mcpTool) Name() string {
	return t.descriptor.FullName
}

func (t *mcpTool) VerbosName() string {
	return fmt.Sprintf("%s / %s", t.descriptor.ServerName, t.descriptor.ToolName)
}

func (t *mcpTool) Description() string {
	return strings.TrimSpace(t.descriptor.Description)
}

func (t *mcpTool) Schema() map[string]interface{} {
	if len(t.descriptor.InputSchema) > 0 {
		return t.descriptor.InputSchema
	}
	return ObjectParamSchema(map[string]interface{}{}, nil)
}

func (t *mcpTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	if err := CheckContext(ctx); err != nil {
		return Result{IsError: true, Name: t.Name()}, err
	}
	manager := mcppkg.GetManager()
	if manager == nil {
		return Result{IsError: true, Name: t.Name()}, fmt.Errorf("mcp manager not initialized")
	}
	callCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	content, err := manager.CallTool(callCtx, t.descriptor.FullName, input)
	if err != nil {
		return Result{IsError: true, Name: t.Name()}, err
	}
	return Result{
		Name:    t.Name(),
		Content: content,
		Title:   t.VerbosName(),
	}, nil
}
