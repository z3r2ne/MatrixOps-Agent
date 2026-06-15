package llmheaders

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

var (
	mu     sync.RWMutex
	custom map[string]string // canonical header -> value
)

// ValidateJSON checks that s is empty or a JSON object with stringifiable values (used before persisting).
func ValidateJSON(s string) error {
	_, err := parseHeadersJSON(s)
	return err
}

// SetFromJSON replaces global headers from a JSON object, e.g. {"X-Custom":"v"}.
// Empty or whitespace-only string clears all custom headers.
func SetFromJSON(s string) error {
	m, err := parseHeadersJSON(s)
	if err != nil {
		return err
	}
	mu.Lock()
	if len(m) == 0 {
		custom = nil
	} else {
		custom = m
	}
	mu.Unlock()
	return nil
}

// Apply sets configured headers on the outgoing request (typically after provider-specific headers).
func Apply(h http.Header) {
	if h == nil {
		return
	}
	mu.RLock()
	defer mu.RUnlock()
	for k, v := range custom {
		if v == "" {
			continue
		}
		h.Set(k, v)
	}
}

func parseHeadersJSON(s string) (map[string]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		switch t := v.(type) {
		case string:
			out[k] = t
		case float64:
			out[k] = trimNumString(t)
		case bool:
			if t {
				out[k] = "true"
			} else {
				out[k] = "false"
			}
		case nil:
			continue
		default:
			b, err := json.Marshal(t)
			if err != nil {
				return nil, fmt.Errorf("header %q: unsupported value type", k)
			}
			out[k] = string(b)
		}
	}
	return out, nil
}

func trimNumString(f float64) string {
	s := fmt.Sprintf("%g", f)
	return strings.TrimRight(strings.TrimRight(s, "0"), ".")
}
