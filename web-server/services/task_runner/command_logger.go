package task_runner

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	database "pkgs/db"
	"pkgs/db/models"

	"gorm.io/gorm"
)

// CommandLogger 命令日志记录器
type CommandLogger struct {
	mu sync.Mutex
	db *gorm.DB
}

type CommandResultUpdate struct {
	Fields   []models.CommandLogField
	ExitCode *int
	Error    error
}

var (
	commandLoggerInstance *CommandLogger
	commandLoggerOnce     sync.Once
)

// GetCommandLogger 获取命令日志记录器单例
func GetCommandLogger(db *gorm.DB) *CommandLogger {
	commandLoggerOnce.Do(func() {
		commandLoggerInstance = &CommandLogger{
			db: db,
		}
	})
	return commandLoggerInstance
}

// LogCommand 记录命令开始执行
func (l *CommandLogger) LogCommand(req models.CommandLogCreate) uint {
	l.mu.Lock()
	defer l.mu.Unlock()

	argsJSON, _ := json.Marshal(req.Args)

	logEntry := models.CommandLog{
		Source:     req.Source,
		SourceID:   req.SourceID,
		SourceName: req.SourceName,
		Command:    req.Command,
		Args:       string(argsJSON),
		WorkDir:    req.WorkDir,
		StdinData:  req.StdinData,
		Fields:     models.MergeCommandLogFields(req.Fields, models.LegacyCommandLogFields(req.StdinData, "", "", "")...),
		Status:     string(models.TaskStatusRunning),
		CreatedAt:  time.Now(),
	}
	_ = logEntry.SyncFields()

	if err := database.CreateCommandLog(l.db, &logEntry); err != nil {
		log.Printf("[CommandLogger] Failed to create log entry: %v", err)
		return 0
	}

	return logEntry.ID
}

// UpdateCommandResult 更新命令执行结果
func (l *CommandLogger) UpdateCommandResult(logID uint, stdout, stderr string, exitCode *int, err error) {
	fields := models.BuildCommandLogFields(
		models.NewCommandLogField("stdout", "输出", truncateString(stdout, 100000), "default"),
		models.NewCommandLogField("stderr", "错误输出", truncateString(stderr, 100000), "error"),
	)
	if err != nil {
		fields = models.MergeCommandLogFields(fields, models.CommandLogField{
			Key:   "error",
			Label: "错误信息",
			Value: err.Error(),
			Tone:  "error",
		})
	}
	l.UpdateCommandResultWithFields(logID, CommandResultUpdate{
		Fields:   fields,
		ExitCode: exitCode,
		Error:    err,
	})
}

func (l *CommandLogger) UpdateCommandResultWithFields(logID uint, result CommandResultUpdate) {
	if logID == 0 {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	updates := map[string]interface{}{
		"exit_code":   result.ExitCode,
		"finished_at": now,
	}

	// 计算执行时长
	if logEntry, logErr := database.GetCommandLogByID(l.db, logID); logErr == nil {
		updates["duration"] = now.Sub(logEntry.CreatedAt).Milliseconds()
		logEntry.Fields = models.MergeCommandLogFields(logEntry.Fields, result.Fields...)
		for _, field := range result.Fields {
			switch field.Key {
			case "stdout":
				logEntry.Stdout = field.Value
				updates["stdout"] = field.Value
			case "stderr":
				logEntry.Stderr = field.Value
				updates["stderr"] = field.Value
			case "stdin":
				logEntry.StdinData = field.Value
				updates["stdin_data"] = field.Value
			case "error":
				logEntry.Error = field.Value
				updates["error"] = field.Value
			}
		}
		if encoded, encodeErr := models.EncodeCommandLogFields(logEntry.Fields); encodeErr == nil {
			updates["fields"] = encoded
		}
	}

	if result.Error != nil {
		updates["error"] = result.Error.Error()
		updates["status"] = "failed"
	} else if result.ExitCode != nil && *result.ExitCode != 0 {
		updates["status"] = "failed"
	} else {
		updates["status"] = "success"
	}

	if dbErr := database.UpdateCommandLogFields(l.db, logID, updates); dbErr != nil {
		log.Printf("[CommandLogger] Failed to update log entry %d: %v", logID, dbErr)
	}
}

// AppendStdout 追加标准输出
func (l *CommandLogger) AppendStdout(logID uint, data string) {
	if logID == 0 || data == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	logEntry, err := database.GetCommandLogByID(l.db, logID)
	if err != nil {
		return
	}
	logEntry.Stdout += data
	logEntry.Fields = models.MergeCommandLogFields(logEntry.Fields, models.CommandLogField{
		Key:   "stdout",
		Label: "输出",
		Value: logEntry.Stdout,
		Tone:  "default",
	})
	encoded, encodeErr := models.EncodeCommandLogFields(logEntry.Fields)
	if encodeErr != nil {
		return
	}
	_ = database.UpdateCommandLogFields(l.db, logID, map[string]interface{}{
		"stdout": logEntry.Stdout,
		"fields": encoded,
	})
}

// AppendStderr 追加标准错误
func (l *CommandLogger) AppendStderr(logID uint, data string) {
	if logID == 0 || data == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	logEntry, err := database.GetCommandLogByID(l.db, logID)
	if err != nil {
		return
	}
	logEntry.Stderr += data
	logEntry.Fields = models.MergeCommandLogFields(logEntry.Fields, models.CommandLogField{
		Key:   "stderr",
		Label: "错误输出",
		Value: logEntry.Stderr,
		Tone:  "error",
	})
	encoded, encodeErr := models.EncodeCommandLogFields(logEntry.Fields)
	if encodeErr != nil {
		return
	}
	_ = database.UpdateCommandLogFields(l.db, logID, map[string]interface{}{
		"stderr": logEntry.Stderr,
		"fields": encoded,
	})
}

// GetLogs 获取命令日志列表
func (l *CommandLogger) GetLogs(query models.CommandLogQuery) ([]models.CommandLog, int64, error) {
	return database.QueryCommandLogs(l.db, query)
}

// GetLog 获取单条命令日志
func (l *CommandLogger) GetLog(id uint) (*models.CommandLog, error) {
	return database.GetCommandLogByID(l.db, id)
}

// ClearOldLogs 清理旧日志（保留最近 N 天）
func (l *CommandLogger) ClearOldLogs(days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	return database.DeleteOldCommandLogs(l.db, cutoff)
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... [truncated]"
}
