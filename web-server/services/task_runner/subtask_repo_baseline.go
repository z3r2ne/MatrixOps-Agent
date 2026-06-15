package task_runner

import (
	"strings"
	"sync"

	coregit "matrixops.local/core_agent/git"
	database "pkgs/db"

	"gorm.io/gorm"
)

var subtaskRepoBaselines sync.Map // taskID -> *coregit.FileSnapshot

func captureSubtaskRepoBaseline(taskID uint, workDir string) {
	workDir = strings.TrimSpace(workDir)
	if taskID == 0 || workDir == "" || !coregit.IsGitRepo(workDir) {
		return
	}
	state, err := coregit.GetRepoState(workDir)
	if err != nil || state == nil {
		return
	}
	snapshot := coregit.FileSnapshotFromRepoState(state)
	if snapshot == nil {
		return
	}
	subtaskRepoBaselines.Store(taskID, snapshot)
}

func storeSubtaskRepoBaselineForTask(dbConn *gorm.DB, taskID uint) {
	if taskID == 0 || dbConn == nil {
		return
	}
	task, err := database.GetTaskByID(dbConn, taskID)
	if err != nil || task == nil || task.ParentTaskID == nil || *task.ParentTaskID == 0 {
		return
	}
	workDir := strings.TrimSpace(task.WorkDir)
	captureSubtaskRepoBaseline(taskID, workDir)
}

func loadSubtaskRepoBaseline(taskID uint) *coregit.FileSnapshot {
	if taskID == 0 {
		return nil
	}
	value, ok := subtaskRepoBaselines.Load(taskID)
	if !ok {
		return nil
	}
	snapshot, _ := value.(*coregit.FileSnapshot)
	return snapshot
}

func clearSubtaskRepoBaseline(taskID uint) {
	if taskID == 0 {
		return
	}
	subtaskRepoBaselines.Delete(taskID)
}

func subtaskFileChangesSinceBaseline(workDir string, baseline *coregit.FileSnapshot) (modified []string, created []string) {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" || !coregit.IsGitRepo(workDir) {
		return nil, nil
	}
	afterState, err := coregit.GetRepoState(workDir)
	if err != nil || afterState == nil {
		return nil, nil
	}
	after := coregit.FileSnapshotFromRepoState(afterState)
	return coregit.DiffFileSnapshots(baseline, after)
}
