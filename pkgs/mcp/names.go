package mcp

import (
	"regexp"
	"strings"
)

var nonNameChars = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func NormalizeName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = nonNameChars.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	return strings.ToLower(value)
}

func BuildToolFullName(serverName, toolName string) string {
	return "mcp__" + NormalizeName(serverName) + "__" + NormalizeName(toolName)
}

func ParseToolFullName(fullName string) (serverName, toolName string, ok bool) {
	if !strings.HasPrefix(fullName, "mcp__") {
		return "", "", false
	}
	parts := strings.SplitN(fullName, "__", 3)
	if len(parts) != 3 || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func IsMcpToolFullName(fullName string) bool {
	_, _, ok := ParseToolFullName(fullName)
	return ok
}
