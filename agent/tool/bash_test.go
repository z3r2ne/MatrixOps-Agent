package tool

import "testing"

func TestAppendTerminalEnvAddsNonInteractivePagerDefaults(t *testing.T) {
	env := appendTerminalEnv([]string{"PATH=/usr/bin"})

	hasTerm := false
	hasGitPager := false
	hasPager := false
	hasLess := false
	for _, entry := range env {
		switch entry {
		case "TERM=xterm-256color":
			hasTerm = true
		case "GIT_PAGER=cat":
			hasGitPager = true
		case "PAGER=cat":
			hasPager = true
		case "LESS=FRX":
			hasLess = true
		}
	}

	if !hasTerm || !hasGitPager || !hasPager || !hasLess {
		t.Fatalf("expected TERM/GIT_PAGER/PAGER/LESS defaults, got %+v", env)
	}
}

func TestAppendTerminalEnvPreservesExistingOverrides(t *testing.T) {
	env := appendTerminalEnv([]string{
		"TERM=screen-256color",
		"GIT_PAGER=less",
		"PAGER=more",
		"LESS=SR",
	})

	counts := map[string]int{}
	for _, entry := range env {
		counts[entry]++
	}

	for _, key := range []string{
		"TERM=screen-256color",
		"GIT_PAGER=less",
		"PAGER=more",
		"LESS=SR",
	} {
		if counts[key] != 1 {
			t.Fatalf("expected to preserve existing env %q exactly once, got %+v", key, env)
		}
	}
}
