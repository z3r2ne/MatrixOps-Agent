package tool

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

var htmlBlockTags = map[string]struct{}{
	"address": {}, "article": {}, "aside": {}, "blockquote": {}, "br": {},
	"dd": {}, "div": {}, "dl": {}, "dt": {}, "fieldset": {}, "figcaption": {},
	"figure": {}, "footer": {}, "form": {}, "h1": {}, "h2": {}, "h3": {}, "h4": {},
	"h5": {}, "h6": {}, "header": {}, "hr": {}, "li": {}, "main": {}, "nav": {},
	"p": {}, "pre": {}, "section": {}, "table": {}, "tbody": {}, "td": {},
	"th": {}, "thead": {}, "tr": {}, "ul": {}, "ol": {},
}

var htmlSkippedTags = map[string]struct{}{
	"script": {}, "style": {}, "noscript": {}, "template": {}, "svg": {}, "head": {},
}

// extractTextFromHTML returns visible text from an HTML document.
func extractTextFromHTML(data []byte) (string, error) {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	var b strings.Builder
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, skip bool) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode {
			if _, ok := htmlSkippedTags[n.Data]; ok {
				skip = true
			}
		}
		if !skip && n.Type == html.TextNode {
			text := strings.TrimSpace(html.UnescapeString(n.Data))
			if text != "" {
				if b.Len() > 0 {
					last := b.String()[b.Len()-1]
					if last != '\n' && last != ' ' {
						b.WriteByte(' ')
					}
				}
				b.WriteString(text)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, skip)
		}
		if !skip && n.Type == html.ElementNode {
			if _, ok := htmlBlockTags[n.Data]; ok {
				b.WriteByte('\n')
			}
		}
	}
	walk(doc, false)

	out := strings.TrimSpace(b.String())
	return collapseBlankLines(out), nil
}

func collapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	compact := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(compact) > 0 && compact[len(compact)-1] == "" {
				continue
			}
			compact = append(compact, "")
			continue
		}
		compact = append(compact, line)
	}
	return strings.TrimSpace(strings.Join(compact, "\n"))
}
