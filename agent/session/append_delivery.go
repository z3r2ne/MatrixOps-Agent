package session

import (
	"fmt"
	"strings"

	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/gorm"
)

// DeliverTaskQueueAppendItem 在 Agent 未运行时将 append 队列项写入会话与记忆，不启动 Agent。
func DeliverTaskQueueAppendItem(db *gorm.DB, task *models.Task, item models.TaskMessageQueueItem) error {
	if db == nil || task == nil {
		return fmt.Errorf("deliver append: db or task is nil")
	}
	if !item.IsAppend() {
		return fmt.Errorf("deliver append: item %q is not append type", item.ID)
	}

	sessionID := strings.TrimSpace(task.SessionID)
	if sessionID == "" {
		return fmt.Errorf("deliver append: task %d has no session", task.ID)
	}

	workerName := strings.TrimSpace(task.WorkerName)
	if workerName == "" {
		workerName = "chat"
	}
	worker, err := database.GetWorkerByName(db, workerName)
	if err != nil || worker == nil {
		worker = &models.Worker{Name: workerName}
	}

	modelSettings, _ := database.GetModelSettingsForWorker(db, worker)
	if modelSettings == nil {
		modelSettings = &models.ModelSettings{Name: "default_model_config"}
	}
	llmConfig := &models.LLMConfig{Name: "default"}
	if worker.LLMConfigID != nil {
		if cfg, cfgErr := database.GetLLMConfigByID(db, *worker.LLMConfigID); cfgErr == nil && cfg != nil {
			llmConfig = cfg
		}
	}

	workDir := strings.TrimSpace(task.WorkDir)
	runner := &AgentRunner{
		db:      db,
		task:    task,
		session: &Info{ID: sessionID, Directory: workDir},
		emitter: NewEmitter(db, sessionID),
	}
	runtimeConfig := &RuntimeConfig{
		Worker:        worker,
		ModelSettings: modelSettings,
		LLMConfig:     llmConfig,
	}
	return runner.deliverImmediateQueueItem(runtimeConfig, item)
}
