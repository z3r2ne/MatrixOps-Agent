package tool

import "testing"

func TestTerminalOutputBufferKeepsFullContent(t *testing.T) {
	buffer := &terminalOutputBuffer{}
	chunk := stringsRepeat("a", 250_000)
	buffer.Append(chunk)

	if got := buffer.Value(); len(got) != len(chunk) {
		t.Fatalf("expected full content length %d, got %d", len(chunk), len(got))
	}
}

func stringsRepeat(s string, count int) string {
	out := make([]byte, 0, len(s)*count)
	for i := 0; i < count; i++ {
		out = append(out, s...)
	}
	return string(out)
}
