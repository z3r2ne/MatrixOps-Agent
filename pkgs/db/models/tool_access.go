package models

import (
	"encoding/json"
	"sort"
	"strings"
)

func ParseEnabledTools(raw string) (map[string]struct{}, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, false, nil
	}

	var names []string
	if err := json.Unmarshal([]byte(trimmed), &names); err != nil {
		return nil, false, err
	}

	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		normalized := strings.TrimSpace(name)
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}
	return set, true, nil
}

func NormalizeEnabledToolsJSON(names []string) string {
	return normalizeEnabledNameListJSON(names)
}

func ParseEnabledSkills(raw string) (map[string]struct{}, bool, error) {
	return ParseEnabledTools(raw)
}

func NormalizeEnabledSkillsJSON(names []string) string {
	return normalizeEnabledNameListJSON(names)
}

func normalizeEnabledNameListJSON(names []string) string {
	normalizedSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		normalizedSet[trimmed] = struct{}{}
	}

	normalized := make([]string, 0, len(normalizedSet))
	for name := range normalizedSet {
		normalized = append(normalized, name)
	}
	sort.Strings(normalized)

	data, _ := json.Marshal(normalized)
	return string(data)
}

func ParseProjectToolPermissions(raw string) (map[string]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]string{}, nil
	}

	var values map[string]string
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, err
	}

	result := make(map[string]string, len(values))
	for name, action := range values {
		if !IsValidProjectToolPermission(action) {
			continue
		}
		result[name] = action
	}
	return result, nil
}

func NormalizeProjectToolPermissionsJSON(values map[string]string) string {
	normalized := make(map[string]string, len(values))
	for name, action := range values {
		if !IsValidProjectToolPermission(action) {
			continue
		}
		normalized[name] = action
	}

	data, _ := json.Marshal(normalized)
	return string(data)
}

func IsValidProjectToolPermission(action string) bool {
	switch action {
	case ProjectToolPermissionAllow, ProjectToolPermissionAsk, ProjectToolPermissionDeny:
		return true
	default:
		return false
	}
}
