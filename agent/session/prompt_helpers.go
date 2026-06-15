package session

import (
	"time"

	"matrixops-agent/util"
	"gorm.io/gorm"
	"pkgs/db/models"
	"pkgs/db/storage"
)

const (
	parentTitlePrefix = "New session - "
	childTitlePrefix  = "Child session - "
)

// getSession 获取会话信息
func getSession(db *gorm.DB, sessionID string) (*Info, error) {
	return storage.GetSession(db, sessionID)
}

// getCustomPromptsWithSource 获取自定义提示词（带来源）
func getCustomPromptsWithSource(task *models.Task) ([][]string, error) {
	return CustomWithSource(task)
}

// generateMessageID 生成消息 ID
func generateMessageID() string {
	return util.Ascending("message")
}

// currentTimeMillis 获取当前时间戳（毫秒）
func currentTimeMillis() int64 {
	return time.Now().UnixMilli()
}

// createDefaultTitle 创建默认的会话标题
func createDefaultTitle(isChild bool) string {
	if isChild {
		return childTitlePrefix + time.Now().Format("2006-01-02 15:04:05")
	}
	return parentTitlePrefix + time.Now().Format("2006-01-02 15:04:05")
}
