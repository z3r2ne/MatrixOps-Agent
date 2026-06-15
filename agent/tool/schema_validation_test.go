package tool

import (
	"fmt"
	"testing"
)

func TestDefaultToolSchemas_ArrayFieldsDeclareItems(t *testing.T) {
	registry := NewDefaultRegistryWithQuestion(&DefaultRegistryOptions{})
	for _, name := range registry.Names() {
		instance, err := registry.Get(name)
		if err != nil {
			t.Fatalf("get tool %s: %v", name, err)
		}
		checkArrayItems(t, name, instance.Schema(), "")
	}
}

func checkArrayItems(t *testing.T, toolName string, value interface{}, path string) {
	t.Helper()

	switch typed := value.(type) {
	case map[string]interface{}:
		if typeName, _ := typed["type"].(string); typeName == "array" {
			if _, ok := typed["items"]; !ok {
				t.Fatalf("tool %s schema path %s has array without items", toolName, displaySchemaPath(path))
			}
		}
		for key, child := range typed {
			nextPath := key
			if path != "" {
				nextPath = path + "." + key
			}
			checkArrayItems(t, toolName, child, nextPath)
		}
	case []interface{}:
		for index, child := range typed {
			nextPath := fmt.Sprintf("%s[%d]", path, index)
			checkArrayItems(t, toolName, child, nextPath)
		}
	}
}

func displaySchemaPath(path string) string {
	if path == "" {
		return "<root>"
	}
	return path
}
