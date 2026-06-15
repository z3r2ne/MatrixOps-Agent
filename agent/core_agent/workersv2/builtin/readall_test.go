package builtin

import (
	"strings"
	"testing"

	"matrixops.local/core_agent/workersv2/generic"
	"matrixops.local/core_agent/workersv2/leader"

	"gopkg.in/yaml.v3"
)

type workerYAML struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	EnabledTools []string `yaml:"enabledTools"`
	SystemPrompt string   `yaml:"systemPrompt"`
	WorkerPrompt string   `yaml:"workerPrompt"`
}

func hasEnabledTool(def workerYAML, name string) bool {
	for _, toolName := range def.EnabledTools {
		if toolName == name {
			return true
		}
	}
	return false
}

func TestReadAllIncludesClawbotWorker(t *testing.T) {
	files := ReadAll()
	data, ok := files["clawbot/clawbot.yaml"]
	if !ok {
		t.Fatalf("clawbot worker yaml missing from builtins")
	}
	var def workerYAML
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal clawbot worker yaml: %v", err)
	}
	if def.Name != "clawbot" {
		t.Fatalf("unexpected worker name: %q", def.Name)
	}
	if !hasEnabledTool(def, "remind") || !hasEnabledTool(def, "message") || !hasEnabledTool(def, "run_worker_task") {
		t.Fatalf("clawbot should include remind, message and run_worker_task, got: %+v", def.EnabledTools)
	}
	if !strings.Contains(def.WorkerPrompt, "300") {
		t.Fatalf("clawbot prompt should enforce 300 char limit")
	}
}

func TestReadAllIncludesExploreWorker(t *testing.T) {
	files := ReadAll()

	data, ok := files["explore/explore.yaml"]
	if !ok {
		t.Fatalf("explore worker yaml missing from builtins")
	}
	if len(data) == 0 {
		t.Fatalf("explore worker yaml is empty")
	}

	var def workerYAML
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal explore worker yaml: %v", err)
	}
	if def.Name != "explore" {
		t.Fatalf("unexpected worker name: %q", def.Name)
	}
	if hasEnabledTool(def, "write") ||
		hasEnabledTool(def, "delete") ||
		hasEnabledTool(def, "edit") ||
		hasEnabledTool(def, "patch") {
		t.Fatalf("explore worker should stay read-only, got tools: %+v", def.EnabledTools)
	}
	if !strings.Contains(def.SystemPrompt, "quick") ||
		!strings.Contains(def.SystemPrompt, "medium") ||
		!strings.Contains(def.SystemPrompt, "very thorough") {
		t.Fatalf("explore worker prompt should describe thoroughness levels")
	}
}

func TestReadAllIncludesCodeMapWorker(t *testing.T) {
	files := ReadAll()

	data, ok := files["code_map/code_map.yaml"]
	if !ok {
		t.Fatalf("code_map worker yaml missing from builtins")
	}
	if len(data) == 0 {
		t.Fatalf("code_map worker yaml is empty")
	}

	var def workerYAML
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal code_map worker yaml: %v", err)
	}
	if def.Name != "code_map" {
		t.Fatalf("unexpected worker name: %q", def.Name)
	}
	if hasEnabledTool(def, "run_worker_task") {
		t.Fatalf("code_map worker should not have run_worker_task, got: %+v", def.EnabledTools)
	}
	if !strings.Contains(def.SystemPrompt, "Recommended code map file") {
		t.Fatalf("code_map prompt should require writing to recommended code map file")
	}
	if !strings.Contains(def.SystemPrompt, "300KB") {
		t.Fatalf("code_map prompt should enforce document size limit")
	}
}

func TestReadAllIncludesPlanWorker(t *testing.T) {
	files := ReadAll()

	data, ok := files["plan/plan.yaml"]
	if !ok {
		t.Fatalf("plan worker yaml missing from builtins")
	}
	if len(data) == 0 {
		t.Fatalf("plan worker yaml is empty")
	}

	var def workerYAML
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal plan worker yaml: %v", err)
	}
	if def.Name != "plan" {
		t.Fatalf("unexpected worker name: %q", def.Name)
	}
	if hasEnabledTool(def, "write") ||
		hasEnabledTool(def, "delete") ||
		hasEnabledTool(def, "edit") ||
		hasEnabledTool(def, "patch") ||
		hasEnabledTool(def, "bash") {
		t.Fatalf("plan worker should stay read-only, got: %+v", def.EnabledTools)
	}
	if !strings.Contains(def.SystemPrompt, "实施步骤") ||
		!strings.Contains(def.SystemPrompt, "关键文件") {
		t.Fatalf("plan worker prompt should require structured planning output")
	}
}

func TestReadAllIncludesVerificationWorker(t *testing.T) {
	files := ReadAll()

	data, ok := files["verification/verification.yaml"]
	if !ok {
		t.Fatalf("verification worker yaml missing from builtins")
	}
	if len(data) == 0 {
		t.Fatalf("verification worker yaml is empty")
	}

	var def workerYAML
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal verification worker yaml: %v", err)
	}
	if def.Name != "verification" {
		t.Fatalf("unexpected worker name: %q", def.Name)
	}
	if hasEnabledTool(def, "write") ||
		hasEnabledTool(def, "delete") ||
		hasEnabledTool(def, "edit") ||
		hasEnabledTool(def, "patch") {
		t.Fatalf("verification worker should not have edit tools, got: %+v", def.EnabledTools)
	}
	if !hasEnabledTool(def, "bash") {
		t.Fatalf("verification worker should have bash for builds/tests, got: %+v", def.EnabledTools)
	}
	if !strings.Contains(def.SystemPrompt, "VERDICT: PASS") ||
		!strings.Contains(def.SystemPrompt, "VERDICT: FAIL") ||
		!strings.Contains(def.SystemPrompt, "VERDICT: PARTIAL") {
		t.Fatalf("verification worker prompt should require explicit verdicts")
	}
}

func TestReadAllIncludesFrontendEngineerWorker(t *testing.T) {
	files := ReadAll()

	data, ok := files["frontend_engineer/frontend_engineer.yaml"]
	if !ok {
		t.Fatalf("frontend_engineer worker yaml missing from builtins")
	}
	if len(data) == 0 {
		t.Fatalf("frontend_engineer worker yaml is empty")
	}

	var def workerYAML
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatalf("unmarshal frontend_engineer worker yaml: %v", err)
	}
	if def.Name != "frontend_engineer" {
		t.Fatalf("unexpected worker name: %q", def.Name)
	}
	if hasEnabledTool(def, "run_worker_task") {
		t.Fatalf("frontend_engineer worker should not have run_worker_task, got: %+v", def.EnabledTools)
	}
	if !strings.Contains(def.SystemPrompt, "选择 `frontend_engineer`") {
		t.Fatalf("frontend_engineer prompt should describe when main worker delegates to this worker")
	}
	if !strings.Contains(def.SystemPrompt, "前端代码改动") {
		t.Fatalf("frontend_engineer prompt should describe frontend implementation scope")
	}
}

func TestMainWorkerPromptsMentionExploreAndVerificationWorkers(t *testing.T) {
	var chat workerYAML
	if err := yaml.Unmarshal(generic.BuiltinChatYAML(), &chat); err != nil {
		t.Fatalf("unmarshal chat yaml: %v", err)
	}
	if !strings.Contains(chat.WorkerPrompt, "<tool_priority>") {
		t.Fatalf("chat worker prompt should delegate tool priority guidance to dynamic prompt layers")
	}

	var lead workerYAML
	if err := yaml.Unmarshal(leader.BuiltinDefinitionYAML(), &lead); err != nil {
		t.Fatalf("unmarshal leader yaml: %v", err)
	}
	if !strings.Contains(lead.SystemPrompt, "<session_guidance>") {
		t.Fatalf("leader system prompt should mention dynamic session guidance")
	}
}
