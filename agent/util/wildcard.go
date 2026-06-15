package util

import (
	"regexp"
	"strings"
)

// Match reports whether value matches the wildcard pattern.
// Supported wildcards: "*" matches any sequence, "?" matches a single rune.
func Match(pattern string, value string) bool {
	if pattern == "*" {
		return true
	}
	re := wildcardToRegexp(pattern)
	return re.MatchString(value)
}

func wildcardToRegexp(pattern string) *regexp.Regexp {
	escaped := regexp.QuoteMeta(pattern)
	escaped = strings.ReplaceAll(escaped, `\*`, ".*")
	escaped = strings.ReplaceAll(escaped, `\?`, ".")
	return regexp.MustCompile("^" + escaped + "$")
}
