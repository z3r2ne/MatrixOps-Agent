package compatible

// ActionSchema 描述兼容模式下模型可输出的 control action（仅用于 compatible ActionProvider prompt 注入）。
type ActionSchema struct {
	ActionName  string
	Description string
	DataSchema  interface{}
}

// SessionActionSchemas 返回兼容模式 prompt 中可用的 action 列表（不含 message）。
func SessionActionSchemas(enableCallToolReason bool) []ActionSchema {
	if enableCallToolReason {
		return []ActionSchema{CallToolWithReasonActionSchema, answerActionSchema}
	}
	return []ActionSchema{CallToolActionSchema, answerActionSchema}
}

// DefaultActionSchemas 非 V2 会话的兼容模式 action 集合。
func DefaultActionSchemas(enableCallToolReason bool) []ActionSchema {
	if enableCallToolReason {
		return []ActionSchema{CallToolWithReasonActionSchema, answerActionSchema}
	}
	return []ActionSchema{CallToolActionSchema, answerActionSchema}
}

var CallToolActionSchema = ActionSchema{
	ActionName:  "call_tool",
	Description: "调用一个或多个工具；当前轮结束后系统会继续把工具结果发回给你，进入下一轮循环",
	DataSchema:  buildCallToolDataSchema(false),
}

var CallToolWithReasonActionSchema = ActionSchema{
	ActionName:  "call_tool",
	Description: "调用一个或多个工具，并为每个工具给出原因（reason）；当前轮结束后系统会继续把工具结果发回给你",
	DataSchema:  buildCallToolDataSchema(true),
}

var answerActionSchema = ActionSchema{
	ActionName:  "answer",
	Description: "回答用户并停止当前任务循环；仅在任务已完成或必须等待用户输入时使用",
	DataSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "最终回答内容。调用 answer 表示当前任务循环应停止。",
			},
		},
		"required":             []string{"content"},
		"additionalProperties": false,
	},
}

func buildCallToolEntrySchema(requireReason bool) map[string]interface{} {
	properties := map[string]interface{}{
		"name": map[string]interface{}{
			"type":        "string",
			"description": "工具名称（见 <tools> 列表）",
		},
		"params": map[string]interface{}{
			"type":        "object",
			"description": "工具参数",
		},
	}
	required := []string{"name", "params"}
	if requireReason {
		properties["reason"] = map[string]interface{}{
			"type":        "string",
			"description": "调用该工具的原因",
		}
		required = append(required, "reason")
	}
	return map[string]interface{}{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

func buildCallToolDataSchema(requireReason bool) map[string]interface{} {
	entrySchema := buildCallToolEntrySchema(requireReason)
	return map[string]interface{}{
		"description": "调用一个或多个工具；工具结果会在后续循环中继续回传给你。",
		"oneOf": []interface{}{
			entrySchema,
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tool_calls": map[string]interface{}{
						"type":     "array",
						"items":    entrySchema,
						"minItems": 1,
					},
				},
				"required": []string{"tool_calls"},
			},
		},
	}
}
