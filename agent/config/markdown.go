package config

import "regexp"

var (
	fileRegex  = regexp.MustCompile("@(\\.?[^\\s`,.]+(?:\\.[^\\s`,.]+)*)")
	shellRegex = regexp.MustCompile("!`([^`]+)`")
)

func MarkdownFiles(template string) [][]string {
	return fileRegex.FindAllStringSubmatch(template, -1)
}

func MarkdownShell(template string) [][]string {
	return shellRegex.FindAllStringSubmatch(template, -1)
}
