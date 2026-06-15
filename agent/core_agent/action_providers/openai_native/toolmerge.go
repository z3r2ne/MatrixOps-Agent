package openai_native

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared/constant"

	"matrixops.local/core_agent/streamtypes"
)

// mergeOpenAINativeToolCalls 合并流式 tool_calls：部分供应商/代理会把「含 name/id 的 delta」与「仅含 arguments 碎片的 delta」
// 放在不同的 tool_calls index 上（例如 index 0 只有函数名、index 1 才流式拼参数）。
// openai-go 的累加器按 index 分槽，导致带名称的那条 arguments 仍为空。
//
// 策略：按槽位顺序遍历；有 function 名的条目开启一条“具名”调用；其后仅有 arguments（且无名称）的槽位
// 将参数拼到最近一条具名调用上。多工具并行且参数错槽时可能歧义，与 OpenAI 官方「同 index 累加」行为相比，
// 本合并优先修复「名称在 K、参数在 K+1」这一类常见分裂。
func mergeOpenAINativeToolCalls(calls []openai.ChatCompletionMessageToolCall) []openai.ChatCompletionMessageToolCall {
	if len(calls) == 0 {
		return calls
	}
	merged := make([]openai.ChatCompletionMessageToolCall, 0, len(calls))
	lastNamed := -1
	for _, tc := range calls {
		name := strings.TrimSpace(tc.Function.Name)
		args := tc.Function.Arguments
		id := strings.TrimSpace(tc.ID)

		if name != "" {
			merged = append(merged, tc)
			lastNamed = len(merged) - 1
			continue
		}

		if args == "" && id == "" {
			continue
		}

		if lastNamed >= 0 && args != "" {
			merged[lastNamed].Function.Arguments += args
			continue
		}

		merged = append(merged, tc)
	}
	return merged
}

// normalizeOpenAINativeToolCalls 在保留 openai-go Accumulator 的前提下，额外修复两类兼容实现异常：
// 1. 名称与 arguments 被拆到不同 index（先由 mergeOpenAINativeToolCalls 处理）。
// 2. 多个工具参数被错误拼进同一条具名调用的 arguments，形成 "{}{}{}" 这样的串接 JSON。
//
// 第二类异常会把后续工具的参数都“吞进”第一条或最近一条具名调用里。这里会：
// - 把 arguments 中串接的多个 JSON 顶层值拆开；
// - 尝试按原有具名槽位顺序重新配对；
// - 若槽位缺名，则按工具 schema 推断名称。
func normalizeOpenAINativeToolCalls(calls []openai.ChatCompletionMessageToolCall, defs []streamtypes.ToolDefinition) []openai.ChatCompletionMessageToolCall {
	merged := mergeOpenAINativeToolCalls(calls)
	repaired, ok := repairOpenAINativeSplitToolCalls(merged, defs)
	if !ok {
		return merged
	}
	return repaired
}

type openAINativeToolPayload struct {
	raw  string
	args map[string]interface{}
}

func repairOpenAINativeSplitToolCalls(calls []openai.ChatCompletionMessageToolCall, defs []streamtypes.ToolDefinition) ([]openai.ChatCompletionMessageToolCall, bool) {
	if len(calls) == 0 {
		return nil, false
	}

	namedSlots := make([]openai.ChatCompletionMessageToolCall, 0, len(calls))
	payloads := make([]openAINativeToolPayload, 0, len(calls))
	needsRepair := false

	for _, tc := range calls {
		if name := strings.TrimSpace(tc.Function.Name); name != "" {
			slot := tc
			slot.Function.Arguments = ""
			namedSlots = append(namedSlots, slot)
		}

		chunks, split, err := splitOpenAINativeArguments(tc.Function.Arguments)
		if err != nil {
			return nil, false
		}
		if split {
			needsRepair = true
		}
		for _, raw := range chunks {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &args); err != nil {
				return nil, false
			}
			payloads = append(payloads, openAINativeToolPayload{
				raw:  raw,
				args: args,
			})
		}
	}

	if !needsRepair || len(payloads) == 0 {
		return nil, false
	}

	unused := append([]openai.ChatCompletionMessageToolCall(nil), namedSlots...)
	out := make([]openai.ChatCompletionMessageToolCall, 0, len(payloads))
	for _, payload := range payloads {
		call, rest, ok := assignOpenAINativePayload(payload, unused, defs)
		if !ok {
			return nil, false
		}
		unused = rest
		call.Function.Arguments = payload.raw
		if call.Type == "" {
			call.Type = constant.Function("function")
		}
		out = append(out, call)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func assignOpenAINativePayload(payload openAINativeToolPayload, unused []openai.ChatCompletionMessageToolCall, defs []streamtypes.ToolDefinition) (openai.ChatCompletionMessageToolCall, []openai.ChatCompletionMessageToolCall, bool) {
	if len(unused) > 0 {
		if scoreOpenAINativeToolArgs(payload.args, findOpenAINativeToolDefinition(unused[0].Function.Name, defs)) >= 0 {
			call := unused[0]
			return call, unused[1:], true
		}
	}

	inferred := inferOpenAINativeToolName(payload.args, defs)
	if inferred != "" {
		if idx := findOpenAINativeNamedSlot(unused, inferred); idx >= 0 {
			call := unused[idx]
			return call, removeOpenAINativeNamedSlot(unused, idx), true
		}
		return openai.ChatCompletionMessageToolCall{
			Type: constant.Function("function"),
			Function: openai.ChatCompletionMessageToolCallFunction{
				Name: inferred,
			},
		}, unused, true
	}

	if len(unused) > 0 {
		call := unused[0]
		return call, unused[1:], true
	}
	return openai.ChatCompletionMessageToolCall{}, unused, false
}

func splitOpenAINativeArguments(raw string) ([]string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false, nil
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	values := make([]string, 0, 1)
	for {
		var msg json.RawMessage
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return nil, false, err
		}
		values = append(values, strings.TrimSpace(string(msg)))
	}
	if len(values) == 0 {
		return nil, false, nil
	}
	return values, len(values) > 1, nil
}

func inferOpenAINativeToolName(args map[string]interface{}, defs []streamtypes.ToolDefinition) string {
	bestName := ""
	bestScore := -1
	for _, def := range defs {
		score := scoreOpenAINativeToolArgs(args, def)
		if score > bestScore {
			bestScore = score
			bestName = strings.TrimSpace(def.Name)
		}
	}
	if bestScore < 0 {
		return ""
	}
	return bestName
}

func scoreOpenAINativeToolArgs(args map[string]interface{}, def streamtypes.ToolDefinition) int {
	name := strings.TrimSpace(def.Name)
	if name == "" {
		return -1
	}

	props := openAINativeSchemaProperties(def.Schema)
	required := openAINativeSchemaRequired(def.Schema)
	if len(args) == 0 {
		if len(required) == 0 {
			return 1
		}
		return -1
	}
	for _, key := range required {
		if _, ok := args[key]; !ok {
			return -1
		}
	}

	matched := 0
	unknown := 0
	for key := range args {
		if _, ok := props[key]; ok {
			matched++
		} else {
			unknown++
		}
	}
	if matched == 0 && len(props) > 0 {
		return -1
	}
	if openAINativeDisallowAdditionalProps(def.Schema) && unknown > 0 {
		return -1
	}

	score := matched*100 - unknown*1000 - (len(props)-matched)*10
	if len(args) == len(props) && len(props) > 0 {
		score += 25
	}
	return score
}

func openAINativeSchemaProperties(schema map[string]interface{}) map[string]interface{} {
	props, _ := schema["properties"].(map[string]interface{})
	if props == nil {
		return map[string]interface{}{}
	}
	return props
}

func openAINativeSchemaRequired(schema map[string]interface{}) []string {
	raw := schema["required"]
	switch typed := raw.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func openAINativeDisallowAdditionalProps(schema map[string]interface{}) bool {
	v, _ := schema["additionalProperties"].(bool)
	return v
}

func findOpenAINativeToolDefinition(name string, defs []streamtypes.ToolDefinition) streamtypes.ToolDefinition {
	name = strings.TrimSpace(name)
	for _, def := range defs {
		if strings.TrimSpace(def.Name) == name {
			return def
		}
	}
	return streamtypes.ToolDefinition{}
}

func findOpenAINativeNamedSlot(calls []openai.ChatCompletionMessageToolCall, name string) int {
	name = strings.TrimSpace(name)
	for idx, tc := range calls {
		if strings.TrimSpace(tc.Function.Name) == name {
			return idx
		}
	}
	return -1
}

func removeOpenAINativeNamedSlot(calls []openai.ChatCompletionMessageToolCall, index int) []openai.ChatCompletionMessageToolCall {
	if index < 0 || index >= len(calls) {
		return calls
	}
	out := make([]openai.ChatCompletionMessageToolCall, 0, len(calls)-1)
	out = append(out, calls[:index]...)
	out = append(out, calls[index+1:]...)
	return out
}
