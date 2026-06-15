package git

import "testing"

func TestDiffFileSnapshotsReportsOnlyNewChanges(t *testing.T) {
	before := &FileSnapshot{
		Modified:  []string{"pre-existing.go"},
		Untracked: []string{"old-untracked.txt"},
	}
	after := &FileSnapshot{
		Modified:  []string{"pre-existing.go", "worker-edited.go"},
		Untracked: []string{"old-untracked.txt", "worker-new.txt"},
	}

	modified, created := DiffFileSnapshots(before, after)
	if len(modified) != 1 || modified[0] != "worker-edited.go" {
		t.Fatalf("modified = %#v, want [worker-edited.go]", modified)
	}
	if len(created) != 1 || created[0] != "worker-new.txt" {
		t.Fatalf("created = %#v, want [worker-new.txt]", created)
	}
}

func TestDiffFileSnapshotsUntrackedPromotedToModified(t *testing.T) {
	before := &FileSnapshot{Untracked: []string{"draft.txt"}}
	after := &FileSnapshot{Modified: []string{"draft.txt"}}

	modified, created := DiffFileSnapshots(before, after)
	if len(modified) != 1 || modified[0] != "draft.txt" {
		t.Fatalf("modified = %#v, want [draft.txt]", modified)
	}
	if len(created) != 0 {
		t.Fatalf("created = %#v, want none", created)
	}
}
