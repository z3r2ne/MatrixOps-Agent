package task_runner

import (
	"strings"
	"testing"

	"matrixops-agent/types"
)

func TestNormalizeMemorySnapshotEntriesRetargetsSessionAndSequence(t *testing.T) {
	entries := normalizeMemorySnapshotEntries("child-session", &types.Memory{
		Entries: []*types.MemoryEntry{
			{
				ID:              11,
				SessionID:       "parent-session",
				SourceMessageID: "msg-parent-1",
				EntryKind:       "text",
				Role:            "user",
				Content:         "parent context",
				Sequence:        7,
				TokenCount:      12,
				Created:         100,
				Updated:         101,
			},
			{
				ID:         12,
				SessionID:  "parent-session",
				EntryKind:  "tool_call",
				Role:       "assistant",
				ToolName:   "read",
				ToolOutput: "file content",
				Sequence:   8,
				Created:    102,
				Updated:    103,
			},
		},
	})

	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	if entries[0].SessionID != "child-session" || entries[1].SessionID != "child-session" {
		t.Fatalf("unexpected session ids: %+v", entries)
	}
	if entries[0].ID != 0 || entries[1].ID != 0 {
		t.Fatalf("expected cloned entries to reset ids: %+v", entries)
	}
	if entries[0].Sequence != 1 || entries[1].Sequence != 2 {
		t.Fatalf("unexpected sequences: %d, %d", entries[0].Sequence, entries[1].Sequence)
	}
	if entries[0].SourceMessageID != "msg-parent-1" {
		t.Fatalf("expected source message id preserved, got %q", entries[0].SourceMessageID)
	}
	if entries[0].TokenCount != 12 {
		t.Fatalf("expected token count preserved, got %d", entries[0].TokenCount)
	}
}

func TestFormatSubtaskCompletionSummaryIncludesKeyFacts(t *testing.T) {
	summary := formatSubtaskCompletionSummary(subtaskCompletionResult{
		TaskID:        12,
		Status:        "failed",
		DurationMs:    1530,
		WorkDir:       "/tmp/worktree",
		Branch:        "feature/subtask",
		CreatedFiles:  []string{"new-file.ts"},
		ModifiedFiles: []string{"src/App.tsx"},
		Error:         "command failed",
		Answer:        "已完成主要实现",
	})

	assertContains := func(fragment string) {
		if !strings.Contains(summary, fragment) {
			t.Fatalf("summary %q missing fragment %q", summary, fragment)
		}
	}

	assertContains("任务 ID: #12")
	assertContains("状态: failed")
	assertContains("执行时长: 1.5s")
	assertContains("工作目录: /tmp/worktree")
	assertContains("新增文件: new-file.ts")
	assertContains("修改文件: src/App.tsx")
	assertContains("错误: command failed")
	assertContains("最终输出: 已完成主要实现")
}

func TestFormatSubtaskCompletionSummaryCancelledIncludesEndReason(t *testing.T) {
	summary := formatSubtaskCompletionSummary(subtaskCompletionResult{
		TaskID: 99,
		Status: "cancelled",
		Error:  "用户取消了任务执行",
	})

	if !strings.Contains(summary, "状态: cancelled") {
		t.Fatalf("summary %q missing cancelled status", summary)
	}
	if !strings.Contains(summary, "结束原因: 用户取消了任务执行") {
		t.Fatalf("summary %q missing end reason for cancellation", summary)
	}
	if strings.Contains(summary, "错误:") {
		t.Fatalf("cancelled summary should not use 错误 line: %q", summary)
	}
}
