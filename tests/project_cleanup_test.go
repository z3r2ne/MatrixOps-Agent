package tests

import (
	"testing"

	database "pkgs/db"
	"pkgs/db/models"

	utils "tests/utils"
)

func TestDeleteProjectRemovesWorkspaceReferenceAndTasks(t *testing.T) {
	db := utils.SetupTestDB(t)

	projectPath := t.TempDir()
	project, err := utils.CreateTestProject(t, db, projectPath)
	if err != nil {
		t.Fatalf("CreateTestProject returned error: %v", err)
	}

	workspace := &models.Workspace{
		Name:       "workspace-1",
		Path:       t.TempDir(),
		ProjectIDs: []uint{project.ID},
	}
	if err := database.CreateWorkspace(db, workspace); err != nil {
		t.Fatalf("CreateWorkspace returned error: %v", err)
	}

	task := &models.Task{
		WorkspaceID: workspace.ID,
		ProjectID:  project.ID,
		Content:    "task-content",
		WorkerName: "chat",
		Status:     "queue",
	}
	if err := database.CreateTask(db, task); err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}

	if err := database.RemoveProjectFromAllWorkspaces(db, project.ID); err != nil {
		t.Fatalf("RemoveProjectFromAllWorkspaces returned error: %v", err)
	}
	if err := database.DeleteTasksByWorkspaceIDAndProjectID(db, workspace.ID, project.ID); err != nil {
		t.Fatalf("DeleteTasksByWorkspaceIDAndProjectID returned error: %v", err)
	}
	if err := database.DeleteProject(db, project.ID); err != nil {
		t.Fatalf("DeleteProject returned error: %v", err)
	}

	workspaceReloaded, err := database.GetWorkspaceByID(db, workspace.ID)
	if err != nil {
		t.Fatalf("GetWorkspaceByID returned error: %v", err)
	}
	if len(workspaceReloaded.ProjectIDs) != 0 {
		t.Fatalf("expected workspace project ids to be empty, got %#v", workspaceReloaded.ProjectIDs)
	}

	tasks, err := database.GetTasksByWorkspaceIDAndProjectID(db, workspace.ID, project.ID)
	if err != nil {
		t.Fatalf("GetTasksByWorkspaceIDAndProjectID returned error: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected no tasks for deleted project, got %d", len(tasks))
	}
}
