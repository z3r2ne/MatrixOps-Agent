package tool

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"pkgs/db/storage"

	"gorm.io/gorm"
)

type PlanItem struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Status string `json:"status"`
}

type PlanDocument struct {
	Request string     `json:"request"`
	Goal    string     `json:"goal"`
	Plan    []PlanItem `json:"plan"`
}

type GetPlanTool struct {
	db *gorm.DB
}

func (GetPlanTool) Name() string {
	return "getplan"
}

func (GetPlanTool) VerbosName() string {
	return "读取计划"
}

func (GetPlanTool) Description() string {
	return "读取会话计划"
}

func (GetPlanTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{}, nil)
}

func (t GetPlanTool) Execute(ctx Context, _ map[string]interface{}) (Result, error) {
	if ctx.SessionID == "" {
		return Result{IsError: true}, errors.New("getplan: sessionID required")
	}
	plan, err := storage.GetPlan(t.db, ctx.SessionID)
	if err != nil {
		return Result{IsError: true}, err
	}
	if plan.Content.Data == nil {
		return Result{Content: "{}"}, nil
	}
	planDoc, err := decodePlanDocument(plan.Content.Data)
	if err != nil {
		return Result{IsError: true}, err
	}
	formatted := formatPlanOutput(planDoc)
	return Result{Content: formatted}, nil
}

type WritePlanTool struct {
	db *gorm.DB
}

func (WritePlanTool) Name() string {
	return "writeplan"
}

func (WritePlanTool) VerbosName() string {
	return "写入计划"
}

func (WritePlanTool) Description() string {
	return "写入会话计划"
}

func (WritePlanTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"plan": map[string]interface{}{
			"type":        "object",
			"description": "Plan document payload",
			"required":    []string{"request", "goal", "plan"},
			"properties": map[string]interface{}{
				"request": map[string]interface{}{
					"type":        "string",
					"description": "Original user request",
				},
				"goal": map[string]interface{}{
					"type":        "string",
					"description": "Target outcome",
				},
				"plan": map[string]interface{}{
					"type":        "array",
					"description": "Plan steps",
					"items": map[string]interface{}{
						"type":     "object",
						"required": []string{"title", "detail"},
						"properties": map[string]interface{}{
							"title": map[string]interface{}{
								"type":        "string",
								"description": "Step title",
							},
							"detail": map[string]interface{}{
								"type":        "string",
								"description": "Step detail",
							},
							"status": map[string]interface{}{
								"type":        "string",
								"description": "Step status",
								"enum":        []string{"pending", "running", "completed", "failed"},
								"default":     "pending",
							},
						},
					},
				},
			},
		},
	}, []string{"plan"})
}

func (t WritePlanTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	if ctx.SessionID == "" {
		return Result{IsError: true}, errors.New("writeplan: sessionID required")
	}
	planValue, ok := input["plan"]
	if !ok {
		return Result{IsError: true}, errors.New("writeplan: missing plan")
	}
	planDoc, raw, err := parsePlanInput(planValue)
	if err != nil {
		return Result{IsError: true}, err
	}
	if err := validatePlan(planDoc); err != nil {
		return Result{IsError: true}, err
	}
	if _, err := storage.UpsertPlan(t.db, ctx.SessionID, raw); err != nil {
		return Result{IsError: true}, err
	}
	encoded, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return Result{IsError: true}, err
	}
	return Result{Content: string(encoded)}, nil
}

type UpdatePlanTool struct {
	db *gorm.DB
}

func (UpdatePlanTool) Name() string {
	return "updateplan"
}

func (UpdatePlanTool) VerbosName() string {
	return "更新计划"
}

func (UpdatePlanTool) Description() string {
	return "更新会话计划的任务状态"
}

func (UpdatePlanTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"index": map[string]interface{}{
			"type":        "integer",
			"description": "Plan item index starting from 0",
		},
		"status": map[string]interface{}{
			"type":        "string",
			"description": "New plan item status",
			"enum":        []string{"pending", "running", "completed", "failed"},
		},
	}, []string{"index", "status"})
}

func (t UpdatePlanTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	if ctx.SessionID == "" {
		return Result{IsError: true}, errors.New("updateplan: sessionID required")
	}
	indexRaw, ok := input["index"]
	if !ok {
		return Result{IsError: true}, errors.New("updateplan: missing index")
	}
	index := intFrom(indexRaw)
	if index < 0 {
		return Result{IsError: true}, errors.New("updateplan: index must be >= 0")
	}
	status := strings.ToLower(strings.TrimSpace(stringFrom(input["status"])))
	if status == "" {
		return Result{IsError: true}, errors.New("updateplan: missing status")
	}
	if err := validatePlanStatus(status); err != nil {
		return Result{IsError: true}, err
	}

	plan, err := storage.GetPlan(t.db, ctx.SessionID)
	if err != nil {
		return Result{IsError: true}, err
	}
	if plan.Content.Data == nil {
		return Result{IsError: true}, errors.New("updateplan: plan is empty")
	}
	planDoc, raw, err := decodePlanContent(plan.Content.Data)
	if err != nil {
		return Result{IsError: true}, err
	}
	if index >= len(planDoc.Plan) {
		return Result{IsError: true}, errors.New("updateplan: index out of range")
	}
	planDoc.Plan[index].Status = status
	if err := updatePlanStatus(raw, index, status); err != nil {
		return Result{IsError: true}, err
	}
	if _, err := storage.UpsertPlan(t.db, ctx.SessionID, raw); err != nil {
		return Result{IsError: true}, err
	}

	encoded, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return Result{IsError: true}, err
	}
	return Result{Content: string(encoded)}, nil
}

func parsePlanInput(value interface{}) (PlanDocument, interface{}, error) {
	var doc PlanDocument
	if value == nil {
		return doc, nil, errors.New("writeplan: plan is empty")
	}
	switch plan := value.(type) {
	case string:
		trimmed := strings.TrimSpace(plan)
		if trimmed == "" {
			return doc, nil, errors.New("writeplan: plan is empty")
		}
		if err := json.Unmarshal([]byte(trimmed), &doc); err != nil {
			return doc, nil, err
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
			return doc, nil, err
		}
		return doc, raw, nil
	case map[string]interface{}:
		encoded, err := json.Marshal(plan)
		if err != nil {
			return doc, nil, err
		}
		if err := json.Unmarshal(encoded, &doc); err != nil {
			return doc, nil, err
		}
		return doc, plan, nil
	default:
		return doc, nil, errors.New("writeplan: plan must be json object")
	}
}

func validatePlan(plan PlanDocument) error {
	if strings.TrimSpace(plan.Request) == "" {
		return errors.New("writeplan: request is required")
	}
	if strings.TrimSpace(plan.Goal) == "" {
		return errors.New("writeplan: goal is required")
	}
	if len(plan.Plan) == 0 {
		return errors.New("writeplan: plan array is required")
	}
	for _, item := range plan.Plan {
		if strings.TrimSpace(item.Title) == "" {
			return errors.New("writeplan: plan item title is required")
		}
		if strings.TrimSpace(item.Detail) == "" {
			return errors.New("writeplan: plan item detail is required")
		}
		if strings.TrimSpace(item.Status) != "" {
			if err := validatePlanStatus(strings.ToLower(strings.TrimSpace(item.Status))); err != nil {
				return errors.New("writeplan: invalid status")
			}
		}
	}
	return nil
}

func decodePlanDocument(data interface{}) (PlanDocument, error) {
	var doc PlanDocument
	if data == nil {
		return doc, errors.New("plan: empty content")
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return doc, err
	}
	if err := json.Unmarshal(encoded, &doc); err != nil {
		return doc, err
	}
	return doc, nil
}

func decodePlanContent(data interface{}) (PlanDocument, map[string]interface{}, error) {
	var doc PlanDocument
	if data == nil {
		return doc, nil, errors.New("plan: empty content")
	}
	switch raw := data.(type) {
	case map[string]interface{}:
		encoded, err := json.Marshal(raw)
		if err != nil {
			return doc, nil, err
		}
		if err := json.Unmarshal(encoded, &doc); err != nil {
			return doc, nil, err
		}
		return doc, raw, nil
	case string:
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return doc, nil, errors.New("plan: empty content")
		}
		if err := json.Unmarshal([]byte(trimmed), &doc); err != nil {
			return doc, nil, err
		}
		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
			return doc, nil, err
		}
		return doc, decoded, nil
	default:
		encoded, err := json.Marshal(raw)
		if err != nil {
			return doc, nil, err
		}
		if err := json.Unmarshal(encoded, &doc); err != nil {
			return doc, nil, err
		}
		var decoded map[string]interface{}
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			return doc, nil, err
		}
		return doc, decoded, nil
	}
}

func updatePlanStatus(raw map[string]interface{}, index int, status string) error {
	if raw == nil {
		return errors.New("updateplan: plan content missing")
	}
	planRaw, ok := raw["plan"]
	if !ok {
		return errors.New("updateplan: plan array missing")
	}
	items, ok := planRaw.([]interface{})
	if !ok {
		return errors.New("updateplan: plan array invalid")
	}
	if index < 0 || index >= len(items) {
		return errors.New("updateplan: index out of range")
	}
	itemRaw, ok := items[index].(map[string]interface{})
	if !ok {
		return errors.New("updateplan: plan item invalid")
	}
	itemRaw["status"] = status
	items[index] = itemRaw
	raw["plan"] = items
	return nil
}

func validatePlanStatus(status string) error {
	switch status {
	case "pending", "running", "completed", "failed":
		return nil
	default:
		return errors.New("plan: invalid status")
	}
}

func formatPlanOutput(plan PlanDocument) string {
	var builder strings.Builder
	builder.WriteString("目标：")
	builder.WriteString(plan.Goal)
	builder.WriteString("\n")
	builder.WriteString("子任务：")
	if len(plan.Plan) == 0 {
		return builder.String()
	}
	builder.WriteString("\n")
	for i, item := range plan.Plan {
		builder.WriteString(strconv.Itoa(i))
		builder.WriteString(": ")
		builder.WriteString(item.Title)
		builder.WriteString(" ")
		builder.WriteString(statusSymbol(item.Status))
		if i < len(plan.Plan)-1 {
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func statusSymbol(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed":
		return "✅"
	case "failed":
		return "❌"
	case "running":
		return "⏳"
	default:
		return "⬜"
	}
}
