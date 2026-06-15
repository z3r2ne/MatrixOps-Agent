package tool

// ObjectParamSchema builds the root JSON Schema for OpenAI function parameters:
// type "object", optional required field names.
func ObjectParamSchema(properties map[string]interface{}, required []string) map[string]interface{} {
	out := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}
