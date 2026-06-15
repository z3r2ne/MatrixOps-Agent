package openai_native

import "testing"

func TestNormalizeOpenAIResponsesSchema_AddsAdditionalPropertiesFalseRecursively(t *testing.T) {
	in := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type": "string",
			},
			"options": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"workdir": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
		"required": []string{"command"},
	}

	got := normalizeOpenAIResponsesSchema(in)

	if got["additionalProperties"] != false {
		t.Fatalf("root additionalProperties = %#v, want false", got["additionalProperties"])
	}
	props, _ := got["properties"].(map[string]interface{})
	options, _ := props["options"].(map[string]interface{})
	if options["additionalProperties"] != false {
		t.Fatalf("nested additionalProperties = %#v, want false", options["additionalProperties"])
	}
}

func TestNormalizeOpenAIResponsesSchema_RequiresAllPropertiesAndMakesOptionalsNullable(t *testing.T) {
	in := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type": "string",
			},
			"depth": map[string]interface{}{
				"type": "number",
			},
		},
		"required": []string{"path"},
	}

	got := normalizeOpenAIResponsesSchema(in)

	required, _ := got["required"].([]interface{})
	if len(required) != 2 || required[0] != "depth" || required[1] != "path" {
		t.Fatalf("required = %#v, want [depth path]", required)
	}

	props, _ := got["properties"].(map[string]interface{})
	pathSchema, _ := props["path"].(map[string]interface{})
	if pathSchema["type"] != "string" {
		t.Fatalf("path type = %#v, want string", pathSchema["type"])
	}
	depthSchema, _ := props["depth"].(map[string]interface{})
	depthTypes, _ := depthSchema["type"].([]interface{})
	if len(depthTypes) != 2 || depthTypes[0] != "number" || depthTypes[1] != "null" {
		t.Fatalf("depth type = %#v, want [number null]", depthSchema["type"])
	}
}
