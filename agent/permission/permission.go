package permission

import "matrixops-agent/util"

type Action string

const (
	Allow Action = "allow"
	Deny  Action = "deny"
	Ask   Action = "ask"
)

type Rule struct {
	Permission string
	Pattern    string
	Action     Action
}

type Ruleset []Rule

func FromConfig(permission map[string]interface{}) Ruleset {
	rules := Ruleset{}
	for key, raw := range permission {
		switch typed := raw.(type) {
		case string:
			rules = append(rules, Rule{
				Permission: key,
				Pattern:    "*",
				Action:     Action(typed),
			})
		case map[string]interface{}:
			for pattern, action := range typed {
				actionStr, ok := action.(string)
				if !ok {
					continue
				}
				rules = append(rules, Rule{
					Permission: key,
					Pattern:    Expand(pattern),
					Action:     Action(actionStr),
				})
			}
		}
	}
	return rules
}

func Merge(rulesets ...Ruleset) Ruleset {
	var merged Ruleset
	for _, ruleset := range rulesets {
		merged = append(merged, ruleset...)
	}
	return merged
}

func Evaluate(permission string, pattern string, rulesets ...Ruleset) Rule {
	merged := Merge(rulesets...)
	for i := len(merged) - 1; i >= 0; i-- {
		rule := merged[i]
		if util.Match(rule.Permission, permission) && util.Match(rule.Pattern, pattern) {
			return rule
		}
	}
	return Rule{
		Permission: permission,
		Pattern:    "*",
		Action:     Ask,
	}
}

var editTools = map[string]bool{
	"edit":      true,
	"write":     true,
	"patch":     true,
	"multiedit": true,
}

// func Disabled(tools []string, ruleset Ruleset) map[string]struct{} {
// 	result := map[string]struct{}{}
// 	for _, tool := range tools {
// 		permission := tool
// 		if editTools[tool] {
// 			permission = "edit"
// 		}
// 		var last Rule
// 		found := false
// 		for i := len(ruleset) - 1; i >= 0; i-- {
// 			rule := ruleset[i]
// 			if util.Match(rule.Permission, permission) {
// 				last = rule
// 				found = true
// 				break
// 			}
// 		}
// 		if found && last.Pattern == "*" && last.Action == Deny {
// 			result[tool] = struct{}{}
// 		}
// 	}
// 	return result
// }

func Disabled(tools []string, ruleset Ruleset) map[string]struct{} {
	result := map[string]struct{}{}
	for _, tool := range tools {
		if !editTools[tool] {
			continue
		}
		permission := "edit"
		var last Rule
		found := false
		for i := len(ruleset) - 1; i >= 0; i-- {
			rule := ruleset[i]
			if util.Match(rule.Permission, permission) {
				last = rule
				found = true
				break
			}
		}
		if found && last.Pattern == "*" && last.Action == Deny {
			result[tool] = struct{}{}
		}
	}
	return result
}
