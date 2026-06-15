package task_runner

import (
	"errors"
	"os"
	"strings"
	"pkgs/db/models"
)

func WorkspaceSessionProjectID(workspaceID string) string {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return ""
	}
	return "workspace:" + workspaceID
}

func parseWorkerConfig(worker models.Worker) (map[string]interface{}, error) {
	cfg := map[string]interface{}{}
	if worker.WorkingDir != "" {
		cfg["working_dir"] = worker.WorkingDir
	}
	if worker.Model != "" {
		cfg["model"] = worker.Model
	}
	if worker.LLMConfigID != nil {
		cfg["llmConfigId"] = *worker.LLMConfigID
	}
	if worker.Occupation != "" {
		cfg["occupation"] = worker.Occupation
	}
	if worker.SystemPrompt != "" {
		cfg["prompt"] = worker.SystemPrompt
	}
	return cfg, nil
}

// func resolveWorkingDir(projectID uint, cfg map[string]interface{}) (string, error) {
// 	if override := toString(cfg["working_dir"]); override != "" {
// 		if stat, err := os.Stat(override); err == nil && stat.IsDir() {
// 			return override, nil
// 		}
// 		return "", errors.New("Worker 工作目录不存在或不可访问")
// 	}

// 	project, err := database.GetProjectByID(projectID)
// 	if err != nil {
// 		return "", errors.New("项目不存在，无法执行任务")
// 	}
// 	if stat, err := os.Stat(project.Path); err != nil || !stat.IsDir() {
// 		return "", errors.New("项目路径不存在或不可访问")
// 	}
// 	return project.Path, nil
// }

// resolveTaskWorkingDir 解析任务的工作目录，优先使用任务的 WorkDir
// func resolveTaskWorkingDir(task models.Task, cfg map[string]interface{}) (string, error) {
// 	// 1. 优先使用任务指定的工作目录（worktree 路径）
// 	if task.WorkDir != "" {
// 		if stat, err := os.Stat(task.WorkDir); err == nil && stat.IsDir() {
// 			return task.WorkDir, nil
// 		}
// 		return "", errors.New("任务工作目录不存在或不可访问: " + task.WorkDir)
// 	}

// 	// 2. 使用 Worker 配置的工作目录
// 	if override := toString(cfg["working_dir"]); override != "" {
// 		if stat, err := os.Stat(override); err == nil && stat.IsDir() {
// 			return override, nil
// 		}
// 		return "", errors.New("Worker 工作目录不存在或不可访问")
// 	}

// 	// 3. 使用项目路径
// 	project, err := database.GetProjectByID(task.ProjectID)
// 	if err != nil {
// 		return "", errors.New("项目不存在，无法执行任务")
// 	}
// 	if stat, err := os.Stat(project.Path); err != nil || !stat.IsDir() {
// 		return "", errors.New("项目路径不存在或不可访问")
// 	}
// 	return project.Path, nil
// }

// resolveTaskWorkingDirFromInfo 解析任务工作目录（基于已解析信息）
func resolveTaskWorkingDirFromInfo(taskWorkDir string, cfg map[string]interface{}, projectPath string) (string, error) {
	if taskWorkDir != "" {
		if stat, err := os.Stat(taskWorkDir); err == nil && stat.IsDir() {
			return taskWorkDir, nil
		}
		return "", errors.New("任务工作目录不存在或不可访问: " + taskWorkDir)
	}
	if override := toString(cfg["working_dir"]); override != "" {
		if stat, err := os.Stat(override); err == nil && stat.IsDir() {
			return override, nil
		}
		return "", errors.New("Worker 工作目录不存在或不可访问")
	}
	if projectPath == "" {
		return "", errors.New("项目路径不存在或不可访问")
	}
	if stat, err := os.Stat(projectPath); err != nil || !stat.IsDir() {
		return "", errors.New("项目路径不存在或不可访问")
	}
	return projectPath, nil
}

func buildCommand(worker models.Worker, cfg map[string]interface{}) (string, []string, error) {
	return "", nil, errors.New("仅支持 matrixops-agent 作为 Worker Provider")
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func assertTask(task *models.Task) error {
	if task == nil {
		return errors.New("任务不存在")
	}
	if task.ID == 0 {
		return errors.New("任务ID不能为0")
	}
	if task.WorkDir == "" {
		return errors.New("任务工作目录不能为空")
	}
	if task.WorkerName == "" {
		return errors.New("任务WorkerName不能为空")
	}
	if task.WorkspaceID == 0 {
		return errors.New("任务WorkspaceID不能为0")
	}
	return nil
}

func assertTaskRuntimeConfig(config *TaskRuntimeConfig) error {
	if config == nil {
		return errors.New("配置不存在")
	}
	if config.TaskID == 0 {
		return errors.New("任务ID不能为0")
	}
	if config.WorkDir == "" {
		return errors.New("WorkDir不能为空")
	}
	return nil
}

func assertTaskRuntimeRunConfig(config *TaskRuntimeConfig) error {
	if config == nil {
		return errors.New("配置不存在")
	}
	if config.Content == "" && len(config.InputParts) == 0 {
		return errors.New("Content不能为空")
	}
	return nil
}

func assertCreateAndRunTaskConfig(config *TaskRuntimeConfig) error {
	if config == nil {
		return errors.New("配置不存在")
	}
	if config.WorkspaceID == "" {
		return errors.New("WorkspaceID不能为空")
	}
	if config.db == nil {
		return errors.New("db不能为nil")
	}
	if config.wsHub == nil {
		return errors.New("wsHub不能为nil")
	}
	if config.Content == "" {
		return errors.New("Content不能为空")
	}
	return nil
}
