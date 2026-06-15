package coreagent

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	actioncompatible "matrixops.local/core_agent/action_providers/compatible"
)

// 本文件只测「模型输出 → ActionOutput」的解析与 data 形态，不调用 HandleAction / Runner。

func readActionDataAll(t *testing.T, a *ActionOutput) []byte {
	t.Helper()
	b, err := io.ReadAll(a.Data)
	if err != nil {
		t.Fatalf("read action data: %v", err)
	}
	return b
}

func sessionV2ActionNameSet() map[string]struct{} {
	names := make(map[string]struct{})
	for _, s := range actioncompatible.SessionActionSchemas(false) {
		names[s.ActionName] = struct{}{}
	}
	for _, s := range actioncompatible.SessionActionSchemas(true) {
		names[s.ActionName] = struct{}{}
	}
	return names
}

func TestParseActionBytes_answer_stringData(t *testing.T) {
	payload := `{"@action":"answer","data":"最终回复"}`
	actions, err := actioncompatible.ParseActionBytes([]byte(payload))
	if err != nil {
		t.Fatalf("ParseActionBytes: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("want 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.Action != "answer" {
		t.Fatalf("Action: got %q", a.Action)
	}
	if !strings.Contains(a.RawJSON, `"@action":"answer"`) {
		t.Fatalf("RawJSON missing envelope: %q", a.RawJSON)
	}
	data := readActionDataAll(t, a)
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("data as string: %v (raw=%q)", err, string(data))
	}
	if s != "最终回复" {
		t.Fatalf("data string: got %q", s)
	}
}

func TestParseActionBytes_missingActionField(t *testing.T) {
	payload := `{"data":"x"}`
	_, err := actioncompatible.ParseActionBytes([]byte(payload))
	if err == nil {
		t.Fatal("expected error for missing @action")
	}
	if !strings.Contains(err.Error(), "@action") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestParseActionBytes_missingDataBecomesNullReader(t *testing.T) {
	payload := `{"@action":"answer"}`
	actions, err := actioncompatible.ParseActionBytes([]byte(payload))
	if err != nil {
		t.Fatalf("ParseActionBytes: %v", err)
	}
	data := strings.TrimSpace(string(readActionDataAll(t, actions[0])))
	if data != "null" {
		t.Fatalf("want data null JSON, got %q", data)
	}
}

func TestParseActionBytes_multipleEnvelopes(t *testing.T) {
	payload := `
{"@action":"call_tool","data":{"name":"read_file","params":{"path":"README.md"}}}
{"@action":"answer","data":"done"}
`
	actions, err := actioncompatible.ParseActionBytes([]byte(payload))
	if err != nil {
		t.Fatalf("ParseActionBytes: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("want 2 actions, got %d", len(actions))
	}
	if actions[0].Action != "call_tool" || actions[1].Action != "answer" {
		t.Fatalf("actions: %q, %q", actions[0].Action, actions[1].Action)
	}
	var ans string
	if err := json.Unmarshal(readActionDataAll(t, actions[1]), &ans); err != nil || ans != "done" {
		t.Fatalf("second data: err=%v ans=%q", err, ans)
	}
}

func TestParseActionBytes_callTool_singleEntry(t *testing.T) {
	payload := `{"@action":"call_tool","data":{"name":"read_file","params":{"path":"/tmp/a"}}}`
	actions, err := actioncompatible.ParseActionBytes([]byte(payload))
	if err != nil {
		t.Fatalf("ParseActionBytes: %v", err)
	}
	data := readActionDataAll(t, actions[0])
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	if string(obj["name"]) != `"read_file"` {
		t.Fatalf("name: %s", obj["name"])
	}
}

func TestParseActionBytes_callTool_toolCallsArray(t *testing.T) {
	payload := `{"@action":"call_tool","data":{"tool_calls":[{"name":"a","params":{}},{"name":"b","params":{}}]}}`
	actions, err := actioncompatible.ParseActionBytes([]byte(payload))
	if err != nil {
		t.Fatalf("ParseActionBytes: %v", err)
	}
	var obj struct {
		ToolCalls []json.RawMessage `json:"tool_calls"`
	}
	if err := json.Unmarshal(readActionDataAll(t, actions[0]), &obj); err != nil {
		t.Fatal(err)
	}
	if len(obj.ToolCalls) != 2 {
		t.Fatalf("tool_calls len: %d", len(obj.ToolCalls))
	}
}

func TestParseActionBytes_parsedNamesMatchSessionV2Handlers(t *testing.T) {
	known := sessionV2ActionNameSet()
	payload := strings.Join([]string{
		`{"@action":"answer","data":"z"}`,
		`{"@action":"call_tool","data":{"name":"t","params":{}}}`,
	}, "\n")
	actions, err := actioncompatible.ParseActionBytes([]byte(payload))
	if err != nil {
		t.Fatalf("ParseActionBytes: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("want 2 actions, got %d", len(actions))
	}
	for _, a := range actions {
		if _, ok := known[a.Action]; !ok {
			t.Fatalf("action %q is not in SessionActionSchemas", a.Action)
		}
	}
}

func TestPrepareActionDataReader_leadingWhitespace(t *testing.T) {
	raw := " \n\t {\"ops\":[]}"
	r, first, err := PrepareActionDataReader(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if first != '{' {
		t.Fatalf("first byte %q", first)
	}
	var obj map[string]json.RawMessage
	if err := json.NewDecoder(r).Decode(&obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["ops"]; !ok {
		t.Fatalf("decoded %v", obj)
	}
}
