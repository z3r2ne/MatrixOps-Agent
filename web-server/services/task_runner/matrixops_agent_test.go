package task_runner

import "testing"

func TestToUintAcceptsUintPointer(t *testing.T) {
	value := uint(153)

	got, ok := toUint(&value)
	if !ok {
		t.Fatal("expected uint pointer to be accepted")
	}
	if got != value {
		t.Fatalf("toUint(&value) = %d, want %d", got, value)
	}
}

func TestToUintRejectsNilUintPointer(t *testing.T) {
	var value *uint

	got, ok := toUint(value)
	if ok {
		t.Fatalf("expected nil uint pointer to be rejected, got %d", got)
	}
}
