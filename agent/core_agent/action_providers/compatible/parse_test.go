package compatible

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"matrixops.local/core_agent/streamtypes"
)

func TestParseActionBytes_callTool(t *testing.T) {
	payload := `{"@action":"call_tool","data":{"name":"read","params":{"path":"/tmp/a"}}}`
	actions, err := ParseActionBytes([]byte(payload))
	if err != nil {
		t.Fatalf("ParseActionBytes: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("want 1 action, got %d", len(actions))
	}
	if actions[0].Action != "call_tool" {
		t.Fatalf("Action: got %q", actions[0].Action)
	}
	data, err := io.ReadAll(actions[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	if string(obj["name"]) != `"read"` {
		t.Fatalf("name: %s", obj["name"])
	}
}

func TestParseActionBytes_answer(t *testing.T) {
	payload := `{"@action":"answer","data":{"content":"done"}}`
	actions, err := ParseActionBytes([]byte(payload))
	if err != nil {
		t.Fatalf("ParseActionBytes: %v", err)
	}
	if len(actions) != 1 || actions[0].Action != "answer" {
		t.Fatalf("unexpected action: %#v", actions[0])
	}
}

func TestParseActionBytes_multipleEnvelopes(t *testing.T) {
	payload := `
{"@action":"call_tool","data":{"name":"read","params":{"path":"README.md"}}}
{"@action":"answer","data":{"content":"done"}}
`
	actions, err := ParseActionBytes([]byte(payload))
	if err != nil {
		t.Fatalf("ParseActionBytes: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("want 2 actions, got %d", len(actions))
	}
	if actions[0].Action != "call_tool" || actions[1].Action != "answer" {
		t.Fatalf("actions: %q, %q", actions[0].Action, actions[1].Action)
	}
}

func TestParseActionBytes_missingAction(t *testing.T) {
	_, err := ParseActionBytes([]byte(`{"data":{}}`))
	if err == nil || !strings.Contains(err.Error(), "@action") {
		t.Fatalf("expected missing @action error, got %v", err)
	}
}

func TestFlatActionDataToActionOutput_rawJSON(t *testing.T) {
	out, err := FlatActionDataToActionOutput("call_tool", []byte(`{"name":"bash","params":{"command":"echo hi"}}`), 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.RawJSON, `"@action":"call_tool"`) {
		t.Fatalf("RawJSON: %q", out.RawJSON)
	}
	if out.Action != "call_tool" {
		t.Fatalf("Action: %q", out.Action)
	}
}

func TestParseActionStream_streaming(t *testing.T) {
	pr, pw := io.Pipe()
	actions := make(chan *streamtypes.ActionOutput, 4)
	var parseErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		ParseActionStream(pr, actions, func(err error) {
			if err != nil {
				parseErr = err
			}
		})
	}()
	go func() {
		_, _ = pw.Write([]byte(`{"@action":"answer","data":{"content":"ok"}}`))
		_ = pw.Close()
	}()
	got := <-actions
	<-done
	if parseErr != nil {
		t.Fatalf("parse error: %v", parseErr)
	}
	if got == nil || got.Action != "answer" {
		t.Fatalf("unexpected action: %#v", got)
	}
}
