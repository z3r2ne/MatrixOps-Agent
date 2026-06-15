package models

import "strings"

// IsCompactionWorker reports whether name identifies the dedicated memory compaction worker.
func IsCompactionWorker(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), WorkerCompaction)
}

// NormalizeCompactionWorkerFields enforces compaction worker invariants.
func NormalizeCompactionWorkerFields(worker *Worker) {
	if worker == nil || !IsCompactionWorker(worker.Name) {
		return
	}
	worker.EnabledTools = NormalizeEnabledToolsJSON(nil)
	worker.EnabledSkills = NormalizeEnabledSkillsJSON(nil)
	worker.Hidden = true
}
