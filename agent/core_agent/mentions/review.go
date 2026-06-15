package mentions

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var reviewMentionRegex = regexp.MustCompile(`\[[^\]]+\]\((review://[^)]+)\)`)

func ExtractReviewMentions(text string) ([]ReviewMention, string) {
	mentions := []ReviewMention{}
	replaced := reviewMentionRegex.ReplaceAllStringFunc(text, func(match string) string {
		sub := reviewMentionRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		raw := sub[1]
		params, ok := ReviewURLToParams(raw)
		if !ok {
			mentions = append(mentions, ReviewMention{RawURL: raw})
			return match
		}
		textValue := ReviewURLToText(raw)
		mentions = append(mentions, ReviewMention{RawURL: raw, Text: textValue, Params: params})
		if textValue == "" {
			return raw
		}
		return textValue
	})
	return mentions, replaced
}

func ReviewURLToParams(raw string) (ReviewParams, bool) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ReviewParams{}, false
	}
	if parsed.Scheme != "review" {
		return ReviewParams{}, false
	}
	query := parsed.Query()
	params := ReviewParams{
		FromType: strings.TrimSpace(query.Get("fromType")),
		From:     strings.TrimSpace(query.Get("from")),
		ToType:   strings.TrimSpace(query.Get("toType")),
		To:       strings.TrimSpace(query.Get("to")),
	}
	if params.FromType == "" || params.From == "" || params.ToType == "" || params.To == "" {
		return ReviewParams{}, false
	}
	return params, true
}

func ReviewURLToText(raw string) string {
	params, ok := ReviewURLToParams(raw)
	if !ok {
		return ""
	}
	return fmt.Sprintf("review from %s:%s to %s:%s", params.FromType, params.From, params.ToType, params.To)
}
