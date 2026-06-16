package tool

import "testing"

func TestIsPermissionExemptTool(t *testing.T) {
	if !IsPermissionExemptTool("set_tool_stall_timeout") {
		t.Fatal("expected set_tool_stall_timeout to be permission exempt")
	}
	if !IsPermissionExemptTool("question") {
		t.Fatal("expected question to be permission exempt")
	}
	if IsPermissionExemptTool("bash") {
		t.Fatal("expected bash not to be permission exempt")
	}
}
