package tool

import "testing"

func TestLoadSkillToolMissingName(t *testing.T) {
	tool := LoadSkillTool{}
	result, err := tool.Execute(Context{}, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !result.IsError {
		t.Fatal("expected result.IsError")
	}
}
