package models

import "testing"

func TestNormalizeCompactionWorkerFields(t *testing.T) {
	worker := &Worker{
		Name:          WorkerCompaction,
		EnabledTools:  `["read","bash"]`,
		EnabledSkills: `["demo"]`,
		Hidden:        false,
	}
	NormalizeCompactionWorkerFields(worker)
	if worker.EnabledTools != "[]" {
		t.Fatalf("enabledTools = %q, want []", worker.EnabledTools)
	}
	if worker.EnabledSkills != "[]" {
		t.Fatalf("enabledSkills = %q, want []", worker.EnabledSkills)
	}
	if !worker.Hidden {
		t.Fatal("expected compaction worker to stay hidden")
	}
}

func TestIsCompactionWorker(t *testing.T) {
	if !IsCompactionWorker("compaction") {
		t.Fatal("expected compaction to match")
	}
	if IsCompactionWorker("chat") {
		t.Fatal("expected chat not to match")
	}
}
