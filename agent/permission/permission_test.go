package permission

import "testing"

func TestEvaluateOrder(t *testing.T) {
	rules := Merge(
		FromConfig(map[string]interface{}{"read": "allow"}),
		FromConfig(map[string]interface{}{"read": map[string]interface{}{"*.env": "deny"}}),
	)
	rule := Evaluate("read", "*.env", rules)
	if rule.Action != Deny {
		t.Fatalf("expected deny, got %s", rule.Action)
	}
}

func TestDisabledEditTools(t *testing.T) {
	rules := FromConfig(map[string]interface{}{"edit": "deny"})
	disabled := Disabled([]string{"write", "read"}, rules)
	if _, ok := disabled["write"]; !ok {
		t.Fatalf("expected write to be disabled")
	}
	if _, ok := disabled["read"]; ok {
		t.Fatalf("expected read to be allowed")
	}
}

func TestFromConfig(t *testing.T) {
	rules := FromConfig(map[string]interface{}{
		"read": map[string]interface{}{
			"*.env": "ask",
		},
	})
	rule := Evaluate("read", "*.env", rules)
	if rule.Action != Ask {
		t.Fatalf("expected ask, got %s", rule.Action)
	}
}
