package handlers

import (
	"testing"

	"pkgs/db/models"
)

func TestSelectTaskBranches(t *testing.T) {
	tests := []struct {
		name          string
		branch        string
		newBranch     string
		baseBranch    string
		currentBranch string
		wantTask      string
		wantBase      string
		wantErr       bool
	}{
		{
			name:          "uses selected branch for normal task",
			branch:        "main",
			currentBranch: "main",
			wantTask:      "main",
			wantBase:      "main",
		},
		{
			name:          "falls back to current branch for normal task",
			currentBranch: "main",
			wantTask:      "main",
			wantBase:      "main",
		},
		{
			name:          "uses explicit base branch for new branch task",
			branch:        "test",
			newBranch:     "feature/test",
			baseBranch:    "main",
			currentBranch: "main",
			wantTask:      "feature/test",
			wantBase:      "main",
		},
		{
			name:          "falls back to selected branch for new branch task",
			branch:        "main",
			newBranch:     "feature/test",
			currentBranch: "main",
			wantTask:      "feature/test",
			wantBase:      "main",
		},
		{
			name:    "errors when no branch can be resolved",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTask, gotBase, err := selectTaskBranches(tt.branch, tt.newBranch, tt.baseBranch, tt.currentBranch)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotTask != tt.wantTask {
				t.Fatalf("task branch = %q, want %q", gotTask, tt.wantTask)
			}
			if gotBase != tt.wantBase {
				t.Fatalf("base branch = %q, want %q", gotBase, tt.wantBase)
			}
		})
	}
}

func TestTaskInputPartsToAgentParts(t *testing.T) {
	got := taskInputPartsToAgentParts([]models.TaskInputPart{
		{
			Type:     "file",
			URL:      "data:image/png;base64,abc",
			Mime:     "image/png",
			Filename: "example.png",
		},
		{
			Type: "file",
		},
		{
			Type: "text",
			URL:  "data:text/plain;base64,abc",
		},
	})

	if len(got) != 1 {
		t.Fatalf("parts len = %d, want 1", len(got))
	}
	if got[0].Type != "file" {
		t.Fatalf("part type = %q, want file", got[0].Type)
	}
	if got[0].URL != "data:image/png;base64,abc" {
		t.Fatalf("part url = %q", got[0].URL)
	}
	if got[0].Mime != "image/png" {
		t.Fatalf("part mime = %q", got[0].Mime)
	}
	if got[0].Filename != "example.png" {
		t.Fatalf("part filename = %q", got[0].Filename)
	}
}

func TestMoveTaskQueueItemToFront(t *testing.T) {
	queue := []models.TaskMessageQueueItem{
		{ID: "a", Content: "first"},
		{ID: "b", Content: "second"},
		{ID: "c", Content: "third"},
	}

	target, next := moveTaskQueueItemToFront(queue, "c")
	if target == nil {
		t.Fatal("expected target item, got nil")
	}
	if target.ID != "c" {
		t.Fatalf("target id = %q, want %q", target.ID, "c")
	}
	if len(next) != 3 {
		t.Fatalf("next queue len = %d, want 3", len(next))
	}
	if next[0].ID != "c" || next[1].ID != "a" || next[2].ID != "b" {
		t.Fatalf("next queue order = %#v", next)
	}
}

func TestMoveTaskQueueItemToFrontReturnsOriginalWhenMissing(t *testing.T) {
	queue := []models.TaskMessageQueueItem{
		{ID: "a", Content: "first"},
		{ID: "b", Content: "second"},
	}

	target, next := moveTaskQueueItemToFront(queue, "missing")
	if target != nil {
		t.Fatalf("expected nil target, got %#v", target)
	}
	if len(next) != len(queue) {
		t.Fatalf("next queue len = %d, want %d", len(next), len(queue))
	}
	for i := range queue {
		if next[i].ID != queue[i].ID {
			t.Fatalf("next[%d].ID = %q, want %q", i, next[i].ID, queue[i].ID)
		}
	}
}

func TestMoveTaskQueueItemToFrontReordersForNextTurn(t *testing.T) {
	queue := []models.TaskMessageQueueItem{
		{ID: "a", Content: "first"},
		{ID: "b", Content: "second"},
		{ID: "c", Content: "third"},
	}

	target, next := moveTaskQueueItemToFront(queue, "c")
	if target == nil {
		t.Fatal("expected target item, got nil")
	}
	if target.ID != "c" {
		t.Fatalf("target id = %q, want %q", target.ID, "c")
	}
	if len(next) != 3 {
		t.Fatalf("next queue len = %d, want 3", len(next))
	}
	if next[0].ID != "c" || next[1].ID != "a" || next[2].ID != "b" {
		t.Fatalf("next queue order = %#v", next)
	}
}
