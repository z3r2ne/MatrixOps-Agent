package tool

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"pkgs/db/storage"
	"pkgs/reminder"

	"gorm.io/gorm"
)

type RemindTool struct {
	db     *gorm.DB
	taskID uint
}

func NewRemindTool(db *gorm.DB, taskID uint) *RemindTool {
	return &RemindTool{db: db, taskID: taskID}
}

var _ Tool = (*RemindTool)(nil)

func (RemindTool) Name() string { return "remind" }

func (RemindTool) VerbosName() string { return "提醒" }

func (RemindTool) Description() string {
	return "管理定时提醒：添加、列出或删除提醒任务。"
}

func (RemindTool) Schema() map[string]interface{} {
	return ObjectParamSchema(map[string]interface{}{
		"action": map[string]interface{}{
			"type":        "string",
			"description": "操作类型",
			"enum":        []string{"add", "list", "remove"},
		},
		"content": map[string]interface{}{
			"type":        "string",
			"description": "提醒内容（add 时必填）",
		},
		"time": map[string]interface{}{
			"type":        "string",
			"description": "时间描述：相对时间如 5m、1h、1h30m、2d；或 cron 如 0 8 * * *、0 9 * * 1-5",
		},
		"jobId": map[string]interface{}{
			"type":        "string",
			"description": "提醒任务 ID（remove 时必填）",
		},
		"name": map[string]interface{}{
			"type":        "string",
			"description": "提醒名称，可选；默认取 content 前 20 字",
		},
	}, []string{"action"})
}

func (t *RemindTool) Execute(ctx Context, input map[string]interface{}) (Result, error) {
	if t.db == nil {
		return Result{IsError: true}, fmt.Errorf("remind: 数据库不可用")
	}
	action := strings.ToLower(strings.TrimSpace(toString(input["action"])))
	switch action {
	case "add":
		return t.add(ctx, input)
	case "list":
		return t.list(ctx)
	case "remove":
		return t.remove(input)
	default:
		return Result{IsError: true}, fmt.Errorf("remind: 未知 action %q", action)
	}
}

func (t *RemindTool) add(ctx Context, input map[string]interface{}) (Result, error) {
	content := strings.TrimSpace(toString(input["content"]))
	timeSpec := strings.TrimSpace(toString(input["time"]))
	if content == "" {
		return Result{IsError: true}, fmt.Errorf("remind: add 需要 content")
	}
	if timeSpec == "" {
		return Result{IsError: true}, fmt.Errorf("remind: add 需要 time")
	}
	name := strings.TrimSpace(toString(input["name"]))
	if name == "" {
		name = reminder.DefaultReminderName(content)
	}
	taskID := t.taskID
	if taskID == 0 {
		return Result{IsError: true}, fmt.Errorf("remind: 当前会话未绑定任务，无法创建提醒")
	}
	job, err := reminder.CreateJob(t.db, taskID, ctx.SessionID, name, content, timeSpec)
	if err != nil {
		return Result{IsError: true}, err
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"jobId":        job.ID,
		"name":         job.Name,
		"content":      job.Content,
		"time":         job.TimeSpec,
		"scheduleKind": job.ScheduleKind,
		"nextRunAt":    formatTimePtr(job.NextRunAt),
	})
	return Result{Content: string(payload), Metadata: map[string]interface{}{"jobId": job.ID}}, nil
}

func (t *RemindTool) list(ctx Context) (Result, error) {
	jobs, err := storage.ListReminderJobs(t.db, t.taskID, ctx.SessionID)
	if err != nil {
		return Result{IsError: true}, err
	}
	items := make([]map[string]interface{}, 0, len(jobs))
	for _, job := range jobs {
		items = append(items, map[string]interface{}{
			"jobId":        job.ID,
			"name":         job.Name,
			"content":      job.Content,
			"time":         job.TimeSpec,
			"scheduleKind": job.ScheduleKind,
			"nextRunAt":    formatTimePtr(job.NextRunAt),
			"status":       job.Status,
		})
	}
	payload, _ := json.Marshal(map[string]interface{}{"items": items, "count": len(items)})
	return Result{Content: string(payload)}, nil
}

func (t *RemindTool) remove(input map[string]interface{}) (Result, error) {
	jobID := strings.TrimSpace(toString(input["jobId"]))
	if jobID == "" {
		return Result{IsError: true}, fmt.Errorf("remind: remove 需要 jobId")
	}
	job, err := storage.GetReminderJobByID(t.db, jobID)
	if err != nil {
		return Result{IsError: true}, fmt.Errorf("remind: 找不到 jobId %q", jobID)
	}
	if t.taskID > 0 && job.TaskID != t.taskID {
		return Result{IsError: true}, fmt.Errorf("remind: jobId 不属于当前任务")
	}
	if err := reminder.RemoveJob(t.db, jobID); err != nil {
		return Result{IsError: true}, err
	}
	payload, _ := json.Marshal(map[string]interface{}{"jobId": jobID, "removed": true})
	return Result{Content: string(payload)}, nil
}

func formatTimePtr(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339)
}

func toString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", value)
	}
}
