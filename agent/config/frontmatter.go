package config

import (
	"os"
	"strings"
)

type Markdown struct {
	Frontmatter map[string]interface{}
	Content     string
}

func ParseMarkdown(path string) (Markdown, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Markdown{}, err
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return Markdown{Frontmatter: map[string]interface{}{}, Content: strings.TrimSpace(text)}, nil
	}

	parts := strings.SplitN(text, "\n", 2)
	if len(parts) < 2 {
		return Markdown{Frontmatter: map[string]interface{}{}, Content: ""}, nil
	}
	rest := parts[1]
	endIdx := strings.Index(rest, "\n---")
	if endIdx == -1 {
		return Markdown{Frontmatter: map[string]interface{}{}, Content: strings.TrimSpace(text)}, nil
	}

	frontmatter := strings.TrimSuffix(rest[:endIdx], "\r")
	content := strings.TrimSpace(rest[endIdx+4:])
	parsed, err := parseYAML(frontmatter)
	if err != nil {
		return Markdown{}, err
	}
	return Markdown{Frontmatter: parsed, Content: content}, nil
}
