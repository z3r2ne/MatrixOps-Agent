package tool

import (
	"encoding/json"
	"strings"
)

const AsyncExecutionParamName = "async"

// WithAsyncExecutionParam 为工具 JSON Schema 增加可选 async 参数。
func WithAsyncExecutionParam(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		schema = ObjectParamSchema(map[string]interface{}{}, nil)
	}
	properties, _ := schema["properties"].(map[string]interface{})
	if properties == nil {
		properties = map[string]interface{}{}
		schema["properties"] = properties
	}
	properties[AsyncExecutionParamName] = map[string]interface{}{
		"type":        "boolean",
		"description": "为 true 时后台异步执行，结果稍后以 async_tool_result 补充消息送达。",
	}
	return schema
}

// ParseAsyncFlag 读取并剥离 async 参数。
func ParseAsyncFlag(input map[string]interface{}) (bool, map[string]interface{}) {
	if len(input) == 0 {
		return false, input
	}
	async := false
	switch value := input[AsyncExecutionParamName].(type) {
	case bool:
		async = value
	case string:
		async = strings.EqualFold(strings.TrimSpace(value), "true")
	}
	if !async {
		return false, input
	}
	stripped := make(map[string]interface{}, len(input)-1)
	for key, value := range input {
		if key == AsyncExecutionParamName {
			continue
		}
		stripped[key] = value
	}
	return true, stripped
}

// IsAsyncEligibleBuiltinTool 判断内置工具是否支持异步执行。
func IsAsyncEligibleBuiltinTool(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if name == "run_worker_task" {
		return true
	}
	for _, info := range CatalogBuiltin() {
		if info.Name == name {
			return true
		}
	}
	return false
}

// ParamsJSONString 将参数序列化为紧凑 JSON 字符串。
func ParamsJSONString(params map[string]interface{}) string {
	if len(params) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
