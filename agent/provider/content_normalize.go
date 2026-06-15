package provider

import "strings"

// SimplifyTextOnlyContent returns a plain string when content consists only of
// text parts. Multimodal content (e.g. images) is returned unchanged.
func SimplifyTextOnlyContent(content interface{}) interface{} {
	switch typed := content.(type) {
	case string:
		return typed
	case []CommonContentPart:
		return simplifyCommonContentParts(typed)
	case []interface{}:
		return simplifyGenericContentParts(typed)
	default:
		return content
	}
}

func simplifyCommonContentParts(parts []CommonContentPart) interface{} {
	if len(parts) == 0 {
		return ""
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		switch strings.TrimSpace(part.Type) {
		case "text":
			if t := strings.TrimSpace(part.Text); t != "" {
				texts = append(texts, t)
			}
		default:
			return parts
		}
	}
	if len(texts) == 0 {
		return ""
	}
	return strings.Join(texts, "\n")
}

func simplifyGenericContentParts(parts []interface{}) interface{} {
	if len(parts) == 0 {
		return ""
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		item, ok := part.(map[string]interface{})
		if !ok {
			return parts
		}
		partType, _ := item["type"].(string)
		switch strings.TrimSpace(partType) {
		case "text":
			if t, ok := item["text"].(string); ok {
				if trimmed := strings.TrimSpace(t); trimmed != "" {
					texts = append(texts, trimmed)
				}
			}
		default:
			return parts
		}
	}
	if len(texts) == 0 {
		return ""
	}
	return strings.Join(texts, "\n")
}
