package coreagent

import (
	"context"
	"fmt"
	"sync"
)

// ActionSchema defines the JSON shape the model may emit.
type ActionSchema struct {
	ActionName  string
	Description string
	DataSchema  interface{}
}

type ActionDataType string

const (
	ActionDataTypeText        ActionDataType = "text"
	ActionDataTypeJSONContent ActionDataType = "json_content"
)

type ActionContext struct {
	Context context.Context
	Runner  *Runner
	State   *RunState
	Prompt  string
	Memory  any
}

const actionCtxToolPartsKey = "tool_parts"
const actionCtxPreAuthorizedToolsKey = "tool_pre_authorized"

func (c *ActionContext) SetToolPart(callID string, part *Part) {
	if c == nil || c.State == nil || callID == "" || part == nil {
		return
	}
	if c.State.Data == nil {
		c.State.Data = map[string]any{}
	}
	stored, _ := c.State.Data[actionCtxToolPartsKey].(*sync.Map)
	if stored == nil {
		stored = &sync.Map{}
		c.State.Data[actionCtxToolPartsKey] = stored
	}
	stored.Store(callID, part)
}

func (c *ActionContext) GetToolPart(callID string) *Part {
	if c == nil || c.State == nil || callID == "" || c.State.Data == nil {
		return nil
	}
	stored, _ := c.State.Data[actionCtxToolPartsKey].(*sync.Map)
	if stored == nil {
		return nil
	}
	value, ok := stored.Load(callID)
	if !ok {
		return nil
	}
	part, _ := value.(*Part)
	return part
}

func (c *ActionContext) preAuthorizedTools() *sync.Map {
	if c == nil || c.State == nil {
		return nil
	}
	if c.State.Data == nil {
		c.State.Data = map[string]any{}
	}
	stored, _ := c.State.Data[actionCtxPreAuthorizedToolsKey].(*sync.Map)
	if stored == nil {
		stored = &sync.Map{}
		c.State.Data[actionCtxPreAuthorizedToolsKey] = stored
	}
	return stored
}

func (c *ActionContext) MarkToolPreAuthorized(callID string) {
	if callID == "" {
		return
	}
	stored := c.preAuthorizedTools()
	if stored == nil {
		return
	}
	stored.Store(callID, true)
}

func (c *ActionContext) IsToolPreAuthorized(callID string) bool {
	if callID == "" {
		return false
	}
	stored := c.preAuthorizedTools()
	if stored == nil {
		return false
	}
	_, ok := stored.Load(callID)
	return ok
}

func (c *ActionContext) NewTextPart(actionName string) *Part {
	part := c.NewPart(PartTypeText)
	part.Metadata = map[string]interface{}{
		"action": actionName,
	}
	return part
}

func (c *ActionContext) NewPart(partType string) *Part {
	part := &Part{
		ID:        c.Runner.nextID("part"),
		MessageID: c.State.Assistant.ID,
		SessionID: c.State.SessionID,
		Type:      partType,
		Time:      &PartTime{Start: c.Runner.now().UnixMilli()},
	}
	return part
}

func (c *ActionContext) NewToolPart(toolName string) *Part {
	now := c.Runner.now().UnixMilli()
	return &Part{
		ID:        c.Runner.nextID("part"),
		MessageID: c.State.Assistant.ID,
		SessionID: c.State.SessionID,
		Type:      PartTypeTool,
		Time:      &PartTime{Start: now, Created: now},
		Tool: &ToolPart{
			Name:   toolName,
			CallID: c.Runner.nextID("tool"),
			State: ToolState{
				Time: PartTime{Start: now, Created: now},
			},
		},
	}
}

func (c *ActionContext) UpdatePart(part *Part) error {
	if part == nil {
		return nil
	}
	updated, err := c.Runner.emitter.UpdatePart(part)
	if err != nil {
		return err
	}
	current := part
	if updated != nil {
		current = updated
	}
	if current.Tool != nil && c.Runner != nil && c.State != nil && c.State.Assistant != nil {
		c.Runner.emitAssistantFooterStatusToolPart(c.State.Assistant.ID, current)
	}
	return err
}

type ActionHandler interface {
	Schema() ActionSchema
	Handle(ctx *ActionContext, action *ActionOutput) ([]*Part, error)
}

type ActionHandlerFunc struct {
	schema ActionSchema
	handle func(ctx *ActionContext, action *ActionOutput) ([]*Part, error)
}

func NewActionHandler(schema ActionSchema, handle func(ctx *ActionContext, action *ActionOutput) ([]*Part, error)) ActionHandler {
	return &ActionHandlerFunc{schema: schema, handle: handle}
}

func (h *ActionHandlerFunc) Schema() ActionSchema {
	return h.schema
}

func (h *ActionHandlerFunc) Handle(ctx *ActionContext, action *ActionOutput) ([]*Part, error) {
	if h.handle == nil {
		return nil, fmt.Errorf("action handler %s has no implementation", h.schema.ActionName)
	}
	return h.handle(ctx, action)
}
