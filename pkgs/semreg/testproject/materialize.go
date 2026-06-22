package testproject

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:fixture
var fixtureFS embed.FS

const FixtureRoot = "fixture"

// MaterializeTo copies the embedded fixture tree into dir (created if missing).
func MaterializeTo(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("target directory is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}
	return fs.WalkDir(fixtureFS, FixtureRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(FixtureRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dir, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fixtureFS.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// MaterializeTemp releases the fixture to a new temp directory.
func MaterializeTemp() (dir string, cleanup func(), err error) {
	dir, err = os.MkdirTemp("", "semreg-fixture-*")
	if err != nil {
		return "", nil, err
	}
	if err := MaterializeTo(dir); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}
