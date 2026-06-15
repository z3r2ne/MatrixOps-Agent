package testrunner

import (
	"fmt"
	"strings"
	"time"

	"matrixops-agent/llm"
	"matrixops-agent/types"
	"gorm.io/gorm"

	database "pkgs/db"
	"pkgs/db/models"
	"pkgs/db/storage"
	taskrunner "web-server/services/task_runner"
)

// TestScenario 定义一个可执行的测试场景
type TestScenario struct {
	ID               string
	Name             string
	Description      string
	TaskInput        string // 发给主 worker 的任务内容
	BuildVerifyInput func(task *models.Task, memoryEntries []*types.MemoryEntry) string
}

// TestResult 测试执行结果
type TestResult struct {
	TaskID             uint      `json:"taskId"`
	VerifyTaskID       uint      `json:"verifyTaskId"`
	Status             string    `json:"status"` // passed / failed / partial / error
	MainTaskOutput     string    `json:"mainTaskOutput"`
	VerificationOutput string    `json:"verificationOutput"`
	Error              string    `json:"error,omitempty"`
	StartedAt          time.Time `json:"startedAt"`
	CompletedAt        time.Time `json:"completedAt"`
}

// Scenarios 预定义测试场景
var Scenarios = map[string]TestScenario{
	"instruction_following": {
		ID:          "instruction_following",
		Name:        "指令遵循",
		Description: "验证 AI 是否能严格按照指令的顺序和内容完成任务",
		TaskInput:   `请按以下顺序执行：1. 在当前目录创建 a.txt 文件；2. 在 a.txt 中写入内容 "123"。`,
		BuildVerifyInput: func(task *models.Task, memoryEntries []*types.MemoryEntry) string {
			return fmt.Sprintf(
				"请验证 AI 是否严格按照指令顺序完成了任务。\n\n"+
					"原始任务目标：先创建 a.txt 文件，再给里面写入 123。\n\n"+
					"执行记录摘要：\n%s\n\n"+
					"请检查：\n"+
					"1. AI 是否先创建了 a.txt 文件\n"+
					"2. AI 是否随后写入了 123\n"+
					"3. 顺序是否正确\n\n"+
					"如果全部满足，请回答 PASS；否则回答 FAIL，并说明原因。",
				formatMemoryForVerification(memoryEntries),
			)
		},
	},
	"complex_task": {
		ID:          "complex_task",
		Name:        "复杂任务",
		Description: "验证 AI 完成多步骤复杂任务的能力",
		TaskInput:   `请完成以下复杂任务：1. 创建目录 test_dir；2. 在 test_dir 下创建 b.txt 和 c.txt；3. 在 b.txt 写入 "hello"，在 c.txt 写入 "world"；4. 将两个文件内容合并到 d.txt 中。`,
		BuildVerifyInput: func(task *models.Task, memoryEntries []*types.MemoryEntry) string {
			return fmt.Sprintf(
				"请验证 AI 是否正确完成了多步骤复杂任务。\n\n"+
					"原始任务目标：创建目录 test_dir，在其中创建 b.txt 和 c.txt 并分别写入 hello 和 world，最后合并到 d.txt。\n\n"+
					"执行记录摘要：\n%s\n\n"+
					"请检查所有步骤是否完成且结果正确。\n"+
					"如果全部满足，请回答 PASS；否则回答 FAIL，并说明原因。",
				formatMemoryForVerification(memoryEntries),
			)
		},
	},
	"exploration": {
		ID:          "exploration",
		Name:        "探索能力",
		Description: "验证 AI 探索代码库并回答问题的能力",
		TaskInput:   `请探索当前代码库，找出项目中所有使用了 "fmt.Printf" 的文件，并列出文件名和所在行号。`,
		BuildVerifyInput: func(task *models.Task, memoryEntries []*types.MemoryEntry) string {
			return fmt.Sprintf(
				"请验证 AI 是否正确完成了代码探索任务。\n\n"+
					"原始任务目标：探索代码库，找出所有使用 fmt.Printf 的文件及行号。\n\n"+
					"执行记录摘要：\n%s\n\n"+
					"请检查 AI 是否：\n"+
					"1. 使用了搜索工具（如 rg/bash）来查找\n"+
					"2. 结果是否准确完整\n\n"+
					"如果满足，请回答 PASS；否则回答 FAIL，并说明原因。",
				formatMemoryForVerification(memoryEntries),
			)
		},
	},
}

// ExecuteScenario 执行单个测试场景（同步阻塞，调用方注意超时）
func ExecuteScenario(
	db *gorm.DB,
	wsHub taskrunner.WSHub,
	llmClient llm.ChatClient,
	workspaceID uint,
	scenario TestScenario,
) (*TestResult, error) {
	result := &TestResult{
		Status:    "running",
		StartedAt: time.Now(),
	}

	// 1. 创建主 task
	workspace, err := database.GetWorkspaceByID(db, workspaceID)
	if err != nil {
		result.Status = "error"
		result.Error = "获取工作区失败: " + err.Error()
		result.CompletedAt = time.Now()
		return result, nil
	}

	mainTask, err := taskrunner.CreateAndRunTask(
		taskrunner.WithDB(db),
		taskrunner.WithWSHub(wsHub),
		taskrunner.WithWorkspaceID(fmt.Sprintf("%d", workspaceID)),
		taskrunner.WithContent(scenario.TaskInput),
		taskrunner.WithTaskName(fmt.Sprintf("[测试] %s", scenario.Name)),
		taskrunner.WithWorkDir(workspace.Path),
		taskrunner.WithLLMClient(llmClient),
	)
	if err != nil {
		result.Status = "error"
		result.Error = "创建测试任务失败: " + err.Error()
		result.CompletedAt = time.Now()
		return result, nil
	}
	result.TaskID = mainTask.ID

	// 2. 启动并等待主 task
	if err := taskrunner.RunTask(mainTask.ID); err != nil {
		result.Status = "error"
		result.Error = "启动测试任务失败: " + err.Error()
		result.CompletedAt = time.Now()
		return result, nil
	}
	if err := taskrunner.WaitTask(mainTask.ID); err != nil {
		result.Status = "error"
		result.Error = "等待测试任务失败: " + err.Error()
		result.CompletedAt = time.Now()
		return result, nil
	}

	// 3. 获取主 task 的 sessionID 和 memory
	mainTask, err = database.GetTaskByID(db, mainTask.ID)
	if err != nil {
		result.Status = "error"
		result.Error = "获取测试任务失败: " + err.Error()
		result.CompletedAt = time.Now()
		return result, nil
	}

	var memoryEntries []*types.MemoryEntry
	if mainTask.SessionID != "" {
		memoryEntries, err = storage.ListMemoryEntriesBySession(db, mainTask.SessionID)
		if err != nil {
			result.Status = "error"
			result.Error = "读取记忆失败: " + err.Error()
			result.CompletedAt = time.Now()
			return result, nil
		}
	}

	// 4. 构造验证输入并创建验证 task
	verifyInput := scenario.BuildVerifyInput(mainTask, memoryEntries)
	result.MainTaskOutput = summarizeMemoryEntries(memoryEntries)

	verifyTask, err := taskrunner.CreateAndRunTask(
		taskrunner.WithDB(db),
		taskrunner.WithWSHub(wsHub),
		taskrunner.WithWorkspaceID(fmt.Sprintf("%d", workspaceID)),
		taskrunner.WithContent(verifyInput),
		taskrunner.WithTaskName(fmt.Sprintf("[验证] %s", scenario.Name)),
		taskrunner.WithWorkDir(workspace.Path),
		taskrunner.WithLLMClient(llmClient),
	)
	if err != nil {
		result.Status = "error"
		result.Error = "创建验证任务失败: " + err.Error()
		result.CompletedAt = time.Now()
		return result, nil
	}
	result.VerifyTaskID = verifyTask.ID

	// 5. 启动并等待验证 task
	if err := taskrunner.RunTask(verifyTask.ID); err != nil {
		result.Status = "error"
		result.Error = "启动验证任务失败: " + err.Error()
		result.CompletedAt = time.Now()
		return result, nil
	}
	if err := taskrunner.WaitTask(verifyTask.ID); err != nil {
		result.Status = "error"
		result.Error = "等待验证任务失败: " + err.Error()
		result.CompletedAt = time.Now()
		return result, nil
	}

	// 6. 读取验证结果
	verifyTask, err = database.GetTaskByID(db, verifyTask.ID)
	if err != nil {
		result.Status = "error"
		result.Error = "获取验证任务失败: " + err.Error()
		result.CompletedAt = time.Now()
		return result, nil
	}

	var verifyOutput string
	if verifyTask.SessionID != "" {
		messages, err := storage.GetMessageWithPartsBySessionIDLight(db, verifyTask.SessionID)
		if err == nil && len(messages) > 0 {
			// 取最后一条 assistant 消息作为验证结果
			for i := len(messages) - 1; i >= 0; i-- {
				if messages[i].Info.Role == "assistant" {
					verifyOutput = extractMessageText(messages[i])
					break
				}
			}
		}
	}
	if verifyOutput == "" {
		verifyOutput = "验证任务未返回结果"
	}
	result.VerificationOutput = verifyOutput

	// 7. 解析 PASS/FAIL
	upper := strings.ToUpper(verifyOutput)
	if strings.Contains(upper, "PASS") && !strings.Contains(upper, "FAIL") {
		result.Status = "passed"
	} else if strings.Contains(upper, "FAIL") {
		result.Status = "failed"
	} else {
		result.Status = "partial"
	}
	result.CompletedAt = time.Now()
	return result, nil
}

func formatMemoryForVerification(entries []*types.MemoryEntry) string {
	var b strings.Builder
	for i, e := range entries {
		if i > 30 {
			b.WriteString("\n... (更多记录已省略)")
			break
		}
		role := e.Role
		if role == "" {
			role = e.EntryKind
		}
		content := e.Content
		if content == "" && e.ToolOutput != "" {
			content = "[Tool Output] " + e.ToolOutput
		}
		if content == "" && e.ToolInputJSON != "" {
			content = "[Tool Input] " + e.ToolInputJSON
		}
		b.WriteString(fmt.Sprintf("[%s] %s\n", role, strings.TrimSpace(content)))
	}
	return b.String()
}

func summarizeMemoryEntries(entries []*types.MemoryEntry) string {
	var b strings.Builder
	for _, e := range entries {
		content := e.Content
		if content == "" && e.ToolOutput != "" {
			content = e.ToolOutput
		}
		if content != "" {
			b.WriteString(content + "\n")
		}
	}
	return b.String()
}

func extractMessageText(msg *types.WithParts) string {
	if msg == nil {
		return ""
	}
	var texts []string
	for _, p := range msg.Parts {
		if p.Type == "text" && p.Text != "" {
			texts = append(texts, p.Text)
		}
	}
	return strings.Join(texts, "\n")
}
