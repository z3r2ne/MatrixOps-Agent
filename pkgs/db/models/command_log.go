package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type CommandLogField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
	Tone  string `json:"tone,omitempty"`
}

// CommandLog 系统命令执行日志
type CommandLog struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	Source     string     `json:"source" gorm:"not null;index"`    // 来源：task_runner, git_handler, etc.
	SourceID   *uint      `json:"sourceId" gorm:"index"`           // 关联 ID（如 taskId, executionId）
	SourceName string     `json:"sourceName"`                      // 来源名称（如任务内容摘要）
	Command    string     `json:"command" gorm:"not null"`         // 命令名
	Args       string     `json:"args"`                            // 参数（JSON 数组）
	WorkDir    string     `json:"workDir"`                         // 工作目录
	StdinData  string     `json:"stdinData" gorm:"type:text"`      // 管道输入数据
	Stdout     string     `json:"stdout" gorm:"type:text"`         // 标准输出
	Stderr     string     `json:"stderr" gorm:"type:text"`         // 标准错误
	ExitCode   *int       `json:"exitCode"`                        // 退出码
	Error      string     `json:"error"`                           // 错误信息
	FieldsRaw  string     `json:"-" gorm:"column:fields;type:text"`
	Fields     []CommandLogField `json:"fields" gorm:"-"`
	Duration   int64      `json:"duration"`                        // 执行时长（毫秒）
	Status     string     `json:"status" gorm:"default:'running'"` // running, success, failed
	CreatedAt  time.Time  `json:"createdAt" gorm:"index"`
	FinishedAt *time.Time `json:"finishedAt"`
}

// CommandLogCreate 创建命令日志的请求
type CommandLogCreate struct {
	Source     string
	SourceID   *uint
	SourceName string
	Command    string
	Args       []string
	WorkDir    string
	StdinData  string
	Fields     []CommandLogField
}

// CommandLogQuery 查询命令日志的参数
type CommandLogQuery struct {
	Source   string `form:"source"`
	SourceID *uint  `form:"sourceId"`
	Status   string `form:"status"`
	Limit    int    `form:"limit"`
	Offset   int    `form:"offset"`
}

func NewCommandLogField(key string, label string, value string, tone string) *CommandLogField {
	if value == "" {
		return nil
	}
	return &CommandLogField{
		Key:   key,
		Label: label,
		Value: value,
		Tone:  tone,
	}
}

func BuildCommandLogFields(fields ...*CommandLogField) []CommandLogField {
	result := make([]CommandLogField, 0, len(fields))
	for _, field := range fields {
		if field == nil {
			continue
		}
		result = append(result, *field)
	}
	return result
}

func LegacyCommandLogFields(stdin string, stdout string, stderr string, errorText string) []CommandLogField {
	return BuildCommandLogFields(
		NewCommandLogField("stdin", "输入", stdin, "default"),
		NewCommandLogField("stdout", "输出", stdout, "default"),
		NewCommandLogField("stderr", "错误输出", stderr, "error"),
		NewCommandLogField("error", "错误信息", errorText, "error"),
	)
}

func MergeCommandLogFields(existing []CommandLogField, updates ...CommandLogField) []CommandLogField {
	if len(updates) == 0 {
		return existing
	}
	merged := append([]CommandLogField(nil), existing...)
	indexByKey := make(map[string]int, len(merged))
	for index, field := range merged {
		indexByKey[field.Key] = index
	}
	for _, update := range updates {
		if update.Key == "" {
			continue
		}
		if update.Value == "" {
			if index, ok := indexByKey[update.Key]; ok {
				merged = append(merged[:index], merged[index+1:]...)
				indexByKey = make(map[string]int, len(merged))
				for idx, field := range merged {
					indexByKey[field.Key] = idx
				}
			}
			continue
		}
		if index, ok := indexByKey[update.Key]; ok {
			merged[index] = update
			continue
		}
		indexByKey[update.Key] = len(merged)
		merged = append(merged, update)
	}
	return merged
}

func EncodeCommandLogFields(fields []CommandLogField) (string, error) {
	if len(fields) == 0 {
		return "", nil
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *CommandLog) SyncFields() error {
	if len(c.Fields) == 0 {
		c.Fields = LegacyCommandLogFields(c.StdinData, c.Stdout, c.Stderr, c.Error)
	}
	encoded, err := EncodeCommandLogFields(c.Fields)
	if err != nil {
		return err
	}
	c.FieldsRaw = encoded
	return nil
}

func (c *CommandLog) BeforeSave(tx *gorm.DB) error {
	return c.SyncFields()
}

func (c *CommandLog) AfterFind(tx *gorm.DB) error {
	if c.FieldsRaw != "" {
		var fields []CommandLogField
		if err := json.Unmarshal([]byte(c.FieldsRaw), &fields); err == nil {
			c.Fields = fields
		}
	}
	if len(c.Fields) == 0 {
		c.Fields = LegacyCommandLogFields(c.StdinData, c.Stdout, c.Stderr, c.Error)
	}
	return nil
}
