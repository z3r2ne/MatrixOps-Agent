package mentions

import (
	"net/url"
	"regexp"
	"strings"
)

var skillMentionRegex = regexp.MustCompile(`\[[^\]]+\]\((skill://[^)]+)\)`)

func ExtractSkillMentionsFromText(text string) ([]SkillMention, string) {
	mentions := []SkillMention{}
	replaced := skillMentionRegex.ReplaceAllStringFunc(text, func(match string) string {
		sub := skillMentionRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		raw := sub[1]
		name := SkillURLToName(raw)
		if name == "" {
			return match
		}
		mentions = append(mentions, SkillMention{RawURL: raw, Name: name})
		return "@" + name
	})
	return mentions, replaced
}

func SkillURLToName(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if parsed.Scheme != "skill" {
		return ""
	}
	name := strings.TrimSpace(parsed.Query().Get("name"))
	if name != "" {
		return name
	}
	return strings.Trim(strings.TrimSpace(parsed.Path), "/")
}
