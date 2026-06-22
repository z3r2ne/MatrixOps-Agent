package testproject

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMaterializeTo(t *testing.T) {
	dir := t.TempDir()
	if err := MaterializeTo(dir); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"README.md", "main.go", "notes.txt", "pkg/greeter/greeter.go"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
}

func TestMaterializeTemp(t *testing.T) {
	dir, cleanup, err := MaterializeTemp()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := os.Stat(filepath.Join(dir, "main.go")); err != nil {
		t.Fatal(err)
	}
}
