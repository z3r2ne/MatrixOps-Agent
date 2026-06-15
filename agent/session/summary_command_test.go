package session

import (
	"strings"
	"testing"

	"matrixops-agent/types"
	"pkgs/db/models"
)

func TestBuildSessionSummaryInstructionFocusesOnProjectIntro(t *testing.T) {
	runner := &AgentRunner{
		session: &types.Info{ID: "session-1"},
		task: &models.Task{
			ProjectID: 1,
			WorkDir:   "/tmp/project",
		},
	}

	instruction := runner.buildSessionSummaryInstruction(&RuntimeConfig{
		Project: &models.Project{Name: "MatrixOps"},
	})

	for _, fragment := range []string{
		"项目介绍",
		"供其他人快速了解项目",
		"项目层面的长期信息",
		"不要把前面对话中的具体任务过程",
	} {
		if !strings.Contains(instruction, fragment) {
			t.Fatalf("expected instruction to contain %q, got:\n%s", fragment, instruction)
		}
	}
}

func TestBuildSessionSummaryTitleInstructionAvoidsTaskStyleTitles(t *testing.T) {
	instruction := buildSessionSummaryTitleInstruction()

	for _, fragment := range []string{
		"项目介绍",
		"项目资料名或项目介绍标题",
		"不要突出某一次具体任务",
	} {
		if !strings.Contains(instruction, fragment) {
			t.Fatalf("expected title instruction to contain %q, got:\n%s", fragment, instruction)
		}
	}
}
