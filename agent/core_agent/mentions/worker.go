package mentions

import (
	"net/url"
	"regexp"
	"strings"
)

var workerMentionRegex = regexp.MustCompile(`\[[^\]]+\]\((worker://[^)]+)\)`)

func ExtractWorkerMentionsFromText(text string) ([]WorkerMention, string) {
	mentions := []WorkerMention{}
	replaced := workerMentionRegex.ReplaceAllStringFunc(text, func(match string) string {
		sub := workerMentionRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		raw := sub[1]
		name := WorkerURLToName(raw)
		if name == "" {
			return match
		}
		mentions = append(mentions, WorkerMention{RawURL: raw, Name: name})
		return "@" + name
	})
	return mentions, replaced
}

func CollectWorkerMentionNames(texts []string) []string {
	seen := map[string]struct{}{}
	workers := []string{}
	for _, text := range texts {
		if strings.TrimSpace(text) == "" {
			continue
		}
		matches := workerMentionRegex.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			name := WorkerURLToName(match[1])
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			workers = append(workers, name)
		}
	}
	return workers
}

func WorkerURLToName(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if parsed.Scheme != "worker" {
		return ""
	}
	name := strings.TrimSpace(parsed.Query().Get("name"))
	if name != "" {
		return name
	}
	return strings.Trim(strings.TrimSpace(parsed.Path), "/")
}
