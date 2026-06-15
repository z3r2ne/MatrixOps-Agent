package ansi

import "regexp"

var csiEscapeRE = regexp.MustCompile(`\x1b\[[\d:;]*[\x40-\x7E]`)

// StripTerminal removes CSI color/style sequences from terminal-oriented tool output (e.g. git).
func StripTerminal(s string) string {
	if s == "" {
		return s
	}
	return csiEscapeRE.ReplaceAllString(s, "")
}
