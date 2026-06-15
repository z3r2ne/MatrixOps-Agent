package session

import "strings"

func joinTextParts(parts []*Part) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == nil {
			continue
		}
		if part.Type == "text" && part.Text != "" {
			out = append(out, part.Text)
		}
		if part.Type == "tool" && part.Tool != nil && part.Tool.State.Output != "" {
			out = append(out, "[tool "+part.Tool.Name+"]\n"+part.Tool.State.Output)
		}
	}
	return strings.Join(out, "\n")
}
