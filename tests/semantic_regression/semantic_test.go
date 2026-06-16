//go:build semanticregression

package semantic_regression

import (
	"strings"
	"testing"

	"pkgs/semreg"
	"pkgs/testrunner"
)

func TestSemanticRegressionSemantic(t *testing.T) {
	if !semregEnabled() {
		t.Skip("set SEMREG_ENABLE=1 to run semantic regression")
	}
	workspaceID, ok := envUint("SEMREG_WORKSPACE_ID")
	if !ok {
		t.Skip("set SEMREG_WORKSPACE_ID to run semantic regression")
	}

	db, err := openSemregDB()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	all, err := testrunner.LoadScenariosFromDir(scenariosDir())
	if err != nil {
		t.Fatal(err)
	}

	items, err := semreg.LoadScenariosDir(scenariosDir())
	if err != nil {
		t.Fatal(err)
	}
	items = semreg.FilterScenarios(items, semreg.TierL2)
	if len(items) == 0 {
		t.Skip("no L2 semantic scenarios configured")
	}

	hub := &traceHub{}
	for _, item := range items {
		if item.Kind != semreg.KindSemantic {
			continue
		}
		scenario, ok := all[item.ID]
		if !ok {
			t.Fatalf("scenario %s not loaded", item.ID)
		}
		t.Run(item.ID, func(t *testing.T) {
			result, err := testrunner.ExecuteScenario(db, hub, nil, workspaceID, scenario)
			if err != nil {
				t.Fatalf("ExecuteScenario: %v", err)
			}
			expect := "passed"
			if item.Semantic != nil && strings.TrimSpace(item.Semantic.ExpectStatus) != "" {
				expect = strings.TrimSpace(item.Semantic.ExpectStatus)
			}
			if result.Status != expect {
				t.Fatalf("status=%s want=%s error=%s verify=%s", result.Status, expect, result.Error, result.VerificationOutput)
			}
		})
	}
}
