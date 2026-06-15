package mentions

import (
	"net/url"
	"regexp"
	"strings"
)

var commandMentionRegex = regexp.MustCompile(`\[[^\]]+\]\((command://[^)]+)\)`)

func ExtractCommandMentionsFromText(text string) ([]CommandMention, string) {
	mentions := []CommandMention{}
	replaced := commandMentionRegex.ReplaceAllStringFunc(text, func(match string) string {
		sub := commandMentionRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		raw := sub[1]
		name := CommandURLToName(raw)
		if name == "" {
			return match
		}
		mentions = append(mentions, CommandMention{RawURL: raw, Name: name})
		return ""
	})
	return mentions, replaced
}

func CommandURLToName(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if parsed.Scheme != "command" {
		return ""
	}
	name := strings.TrimSpace(parsed.Query().Get("name"))
	if name != "" {
		return name
	}
	return strings.Trim(strings.TrimSpace(parsed.Path), "/")
}
