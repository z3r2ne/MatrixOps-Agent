package session

import (
	"errors"
	"fmt"

	"matrixops-agent/tool"
	database "pkgs/db"
	"pkgs/db/storage"

	"gorm.io/gorm"
)

// buildStopWorkerTaskFunc 构建停止子任务的回调函数
func buildStopWorkerTaskFunc(db *gorm.DB, parentTaskID uint) tool.StopWorkerTaskFunc {
	if db == nil {
		return nil
	}

	return func(ctx tool.Context, taskID uint) error {
		if taskID == 0 {
			return errors.New("task_id is required")
		}

		// 验证任务存在且是当前任务的子任务
		task, err := database.GetTaskByID(db, taskID)
		if err != nil {
			return fmt.Errorf("task %d not found: %w", taskID, err)
		}
		if task == nil {
			return fmt.Errorf("task %d not found", taskID)
		}

		// 验证是子任务
		if parentTaskID == 0 {
			return errors.New("parent task context is required")
		}
		if task.ParentTaskID == nil || *task.ParentTaskID != parentTaskID {
			return fmt.Errorf("task %d is not a subtask of current task %d", taskID, parentTaskID)
		}

		// 调用 CancelTask
		if err := database.UpdateTaskFields(db, taskID, map[string]interface{}{
			"status": "cancelled",
			"error":  "stopped by parent task",
		}); err != nil {
			return fmt.Errorf("update task status: %w", err)
		}

		return nil
	}
}

// buildGetWorkerTaskProgressFunc 构建获取子任务进度的回调函数
func buildGetWorkerTaskProgressFunc(db *gorm.DB, parentTaskID uint) tool.GetWorkerTaskProgressFunc {
	if db == nil {
		return nil
	}

	return func(ctx tool.Context, taskID uint, limit int) (*tool.WorkerTaskProgressResult, error) {
		if taskID == 0 {
			return nil, errors.New("task_id is required")
		}

		// 获取任务信息
		task, err := database.GetTaskByID(db, taskID)
		if err != nil {
			return nil, fmt.Errorf("task %d not found: %w", taskID, err)
		}
		if task == nil {
			return nil, fmt.Errorf("task %d not found", taskID)
		}

		// 验证是子任务
		if parentTaskID > 0 {
			if task.ParentTaskID == nil || *task.ParentTaskID != parentTaskID {
				return nil, fmt.Errorf("task %d is not a subtask of current task %d", taskID, parentTaskID)
			}
		}

		result := &tool.WorkerTaskProgressResult{
			TaskID:     task.ID,
			SessionID:  task.SessionID,
			Status:     task.Status,
			WorkerName: task.WorkerName,
			TaskName:   task.Name,
			Content:    task.Content,
		}

		// 获取会话消息
		if task.SessionID != "" && limit > 0 {
			messages, _, _, err := storage.GetMessageWithPartsBySessionIDPageLight(db, task.SessionID, limit, "")
			if err == nil {
				for _, msg := range messages {
					if msg.Info == nil {
						continue
					}
					progressMsg := tool.WorkerTaskProgressMessage{
						ID:         msg.Info.ID,
						Role:       string(msg.Info.Role),
						WorkerName: msg.Info.Worker,
					}
					for _, part := range msg.Parts {
						partMap := map[string]interface{}{
							"id":   part.ID,
							"type": string(part.Type),
						}
						if part.Text != "" {
							partMap["text"] = truncateText(part.Text, 500)
						}
						progressMsg.Parts = append(progressMsg.Parts, partMap)
					}
					result.Messages = append(result.Messages, progressMsg)
				}
			}
		}

		return result, nil
	}
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
