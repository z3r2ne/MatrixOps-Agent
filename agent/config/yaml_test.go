package config

import (
	"os"
	"path/filepath"
	"testing"

	"matrixops-agent/global"
	"pkgs/db/models"
)

func TestYAMLConfig(t *testing.T) {
	dir := t.TempDir()
	global.ResetForTest()
	t.Setenv(global.EnvTestHome, dir)

	configPath := filepath.Join(dir, global.AppName+".yaml")
	content := "key: test-key\nmodel: openai/gpt-4o-mini\nbaseurl: https://example.test/v1\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	task := &models.Task{ProjectID: 1, WorkDir: dir}
	cfg, err := Get(task)
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if cfg.Model != "openai/gpt-4o-mini" {
		t.Fatalf("model mismatch: %s", cfg.Model)
	}
	openai := cfg.Provider["openai"]
	if openai.Options["apiKey"] != "test-key" {
		t.Fatalf("apiKey mismatch: %#v", openai.Options["apiKey"])
	}
	if openai.Options["baseURL"] != "https://example.test/v1" {
		t.Fatalf("baseURL mismatch: %#v", openai.Options["baseURL"])
	}
}
