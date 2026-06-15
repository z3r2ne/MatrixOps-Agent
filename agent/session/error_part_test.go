package session

import "testing"

func TestNewAssistantErrorPart(t *testing.T) {
	assistant := &MessageInfo{
		ID:        "msg-1",
		SessionID: "sess-1",
	}
	messageError := &MessageError{
		Name:    "llm_error",
		Message: "stream failed",
	}

	part := newAssistantErrorPart(assistant, messageError)
	if part == nil {
		t.Fatal("expected non-nil part")
	}
	if part.Type != "error" {
		t.Fatalf("type = %q, want error", part.Type)
	}
	if part.MessageID != assistant.ID || part.SessionID != assistant.SessionID {
		t.Fatalf("part ids = (%q,%q)", part.MessageID, part.SessionID)
	}
	if part.Error != messageError {
		t.Fatal("expected error payload to be attached")
	}
	if part.Text != "stream failed" {
		t.Fatalf("text = %q", part.Text)
	}
	if part.Time == nil || part.Time.Start == 0 || part.Time.End == 0 {
		t.Fatalf("expected timestamps, got %+v", part.Time)
	}
}
