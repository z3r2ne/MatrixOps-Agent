package compatible

import (
	"strings"
	"testing"
)

func TestSessionActionSchemasExcludesMessage(t *testing.T) {
	schemas := SessionActionSchemas(false)

	if len(schemas) != 2 {
		t.Fatalf("expected 2 action schemas, got %d", len(schemas))
	}

	if schemas[0].ActionName != "call_tool" {
		t.Fatalf("expected first schema to be call_tool, got %q", schemas[0].ActionName)
	}
	if schemas[1].ActionName != "answer" {
		t.Fatalf("expected second schema to be answer, got %q", schemas[1].ActionName)
	}
}

func TestCallToolDataSchemaUsesNameAndParams(t *testing.T) {
	dataSchema, ok := CallToolActionSchema.DataSchema.(map[string]interface{})
	if !ok {
		t.Fatal("expected call_tool data schema to be object")
	}
	options, ok := dataSchema["oneOf"].([]interface{})
	if !ok || len(options) == 0 {
		t.Fatalf("expected oneOf options, got %#v", dataSchema["oneOf"])
	}
	single, ok := options[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected single call_tool option")
	}
	properties, ok := single["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties")
	}
	if _, ok := properties["name"]; !ok {
		t.Fatal("expected name property")
	}
	if _, ok := properties["params"]; !ok {
		t.Fatal("expected params property")
	}
}

func TestBuildActionEnvelopeJSON(t *testing.T) {
	got := BuildActionEnvelopeJSON("call_tool", []byte(`{"name":"read","params":{"path":"a"}}`))
	if !strings.Contains(got, `"@action":"call_tool"`) {
		t.Fatalf("unexpected envelope: %s", got)
	}
	if !strings.Contains(got, `"name":"read"`) {
		t.Fatalf("unexpected envelope: %s", got)
	}
}
