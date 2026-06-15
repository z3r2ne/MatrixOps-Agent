package permission

import (
	"errors"
	"os"
	"strings"
)

type Request struct {
	ID         string
	SessionID  string
	Permission string
	Patterns   []string
	Metadata   map[string]interface{}
	Always     []string
}

type Reply string

const (
	ReplyOnce   Reply = "once"
	ReplyAlways Reply = "always"
	ReplyReject Reply = "reject"
)

type Manager struct {
	pending  map[string]Request
	approved Ruleset
}

func NewManager(approved Ruleset) *Manager {
	return &Manager{
		pending:  map[string]Request{},
		approved: approved,
	}
}

func (m *Manager) Ask(request Request, ruleset Ruleset) error {
	for _, pattern := range request.Patterns {
		rule := Evaluate(request.Permission, pattern, ruleset, m.approved)
		switch rule.Action {
		case Deny:
			return DeniedError{Ruleset: ruleset}
		case Ask:
			m.pending[request.ID] = request
			return AskError{Request: request}
		case Allow:
			continue
		}
	}
	return nil
}

func (m *Manager) Reply(requestID string, reply Reply) error {
	request, ok := m.pending[requestID]
	if !ok {
		return errors.New("permission request not found")
	}
	delete(m.pending, requestID)

	switch reply {
	case ReplyReject:
		return RejectedError{}
	case ReplyOnce:
		return nil
	case ReplyAlways:
		for _, pattern := range request.Always {
			m.approved = append(m.approved, Rule{
				Permission: request.Permission,
				Pattern:    pattern,
				Action:     Allow,
			})
		}
		return nil
	}
	return nil
}

func (m *Manager) Pending() []Request {
	out := make([]Request, 0, len(m.pending))
	for _, req := range m.pending {
		out = append(out, req)
	}
	return out
}

type AskError struct {
	Request Request
}

func (e AskError) Error() string {
	return "permission requires approval"
}

type RejectedError struct{}

func (e RejectedError) Error() string {
	return "permission request rejected"
}

type DeniedError struct {
	Ruleset Ruleset
}

func (e DeniedError) Error() string {
	return "permission denied by rule"
}

func Covered(permission string, pattern string, rules Ruleset) bool {
	return Evaluate(permission, pattern, rules).Action == Allow
}

func Expand(pattern string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(pattern, "~/") {
		return home + pattern[1:]
	}
	if pattern == "~" {
		return home
	}
	if strings.HasPrefix(pattern, "$HOME/") {
		return home + pattern[5:]
	}
	if strings.HasPrefix(pattern, "$HOME") {
		return home + pattern[5:]
	}
	return pattern
}
