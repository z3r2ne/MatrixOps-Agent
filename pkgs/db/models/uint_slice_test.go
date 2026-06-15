package models

import (
	"encoding/json"
	"testing"
)

func TestUintSliceUnmarshalJSON(t *testing.T) {
	t.Run("array", func(t *testing.T) {
		var ids UintSlice
		if err := json.Unmarshal([]byte("[1,2]"), &ids); err != nil {
			t.Fatalf("unmarshal array: %v", err)
		}
		if got := []uint(ids); len(got) != 2 || got[0] != 1 || got[1] != 2 {
			t.Fatalf("unexpected ids: %#v", got)
		}
	})

	t.Run("single number legacy", func(t *testing.T) {
		var ids UintSlice
		if err := json.Unmarshal([]byte("5"), &ids); err != nil {
			t.Fatalf("unmarshal number: %v", err)
		}
		if got := []uint(ids); len(got) != 1 || got[0] != 5 {
			t.Fatalf("unexpected ids: %#v", got)
		}
	})
}
