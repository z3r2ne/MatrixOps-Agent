package mentions

import (
	"net/url"
	"path/filepath"
	"regexp"
)

var fileMentionRegex = regexp.MustCompile(`\[[^\]]+\]\((file://[^)]+)\)`)

func ExtractFileMentions(text string) ([]FileMention, string) {
	mentions := []FileMention{}
	replaced := fileMentionRegex.ReplaceAllStringFunc(text, func(match string) string {
		sub := fileMentionRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		raw := sub[1]
		path := FileURLToPath(raw)
		mentions = append(mentions, FileMention{RawURL: raw, Path: path})
		if path == "" {
			return raw
		}
		return path
	})
	return mentions, replaced
}

func FileURLToPath(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	filePath := parsed.Query().Get("filePath")
	if filePath == "" {
		return ""
	}
	if !filepath.IsAbs(filePath) {
		filePath = "./" + filePath
	}
	return filePath
}
