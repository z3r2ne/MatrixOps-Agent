package models

import (
	"database/sql/driver"
	"encoding/json"

	"matrixops-agent/permission"
)

// Session 会话数据库模型
type Session struct {
	ID            string `gorm:"primaryKey;size:255"`
	Slug          string `gorm:"size:255;index"`
	ProjectID     string `gorm:"size:255;index;not null"`
	Directory     string `gorm:"type:text"`
	WorkspaceRoot string `gorm:"type:text"`
	WorkspacePath string `gorm:"type:text"`
	ParentID      string `gorm:"size:255;index"`
	Title         string `gorm:"type:text"`
	Version       string `gorm:"size:100"`
	StartSnapshot string `gorm:"size:255"`

	// 时间字段
	Created    int64 `gorm:"index"`
	Updated    int64 `gorm:"index"`
	Compacting int64
	Archived   int64 `gorm:"index"`

	// JSON 字段
	Summary         JSONField `gorm:"type:text"`
	MemoryAnalysis  JSONField `gorm:"type:text;column:memory_analysis"`
	CriticalInfo    JSONField `gorm:"type:text;column:critical_info"`
	Share           JSONField `gorm:"type:text"`
	Permission    JSONField `gorm:"type:text"`
	Revert        JSONField `gorm:"type:text"`
	Tokens        JSONField `gorm:"type:text"`
	EnabledSkills JSONField `gorm:"type:text"`
}

// TableName 指定表名
func (Session) TableName() string {
	return "sessions"
}

// SessionSummary 会话摘要
type SessionSummary struct {
	Additions int         `json:"additions"`
	Deletions int         `json:"deletions"`
	Files     int         `json:"files"`
	Diffs     interface{} `json:"diffs,omitempty"`
}

// SessionShare 分享信息
type SessionShare struct {
	URL string `json:"url"`
}

// SessionRevert 回退信息
type SessionRevert struct {
	MessageID string `json:"messageID"`
	PartID    string `json:"partID,omitempty"`
	Snapshot  string `json:"snapshot,omitempty"`
	Diff      string `json:"diff,omitempty"`
}

// ToTypesInfo 转换为 types.Info
func (s *Session) ToTypesInfo() interface{} {
	return map[string]interface{}{
		"id":            s.ID,
		"slug":          s.Slug,
		"projectID":     s.ProjectID,
		"directory":     s.Directory,
		"workspaceRoot": s.WorkspaceRoot,
		"workspacePath": s.WorkspacePath,
		"enabledSkills": s.EnabledSkills.Data,
		"parentID":      s.ParentID,
		"title":         s.Title,
		"version":       s.Version,
		"summary":       s.Summary.Data,
		"share":         s.Share.Data,
		"permission":    s.Permission.Data,
		"revert":        s.Revert.Data,
		"tokens":        s.Tokens.Data,
		"time": map[string]interface{}{
			"created":    s.Created,
			"updated":    s.Updated,
			"compacting": s.Compacting,
			"archived":   s.Archived,
		},
	}
}

// FromTypesInfo 从 types.Info 创建
func SessionFromMap(data map[string]interface{}) *Session {
	s := &Session{}

	if v, ok := data["id"].(string); ok {
		s.ID = v
	}
	if v, ok := data["slug"].(string); ok {
		s.Slug = v
	}
	if v, ok := data["projectID"].(string); ok {
		s.ProjectID = v
	}
	if v, ok := data["directory"].(string); ok {
		s.Directory = v
	}
	if v, ok := data["workspaceRoot"].(string); ok {
		s.WorkspaceRoot = v
	}
	if v, ok := data["workspacePath"].(string); ok {
		s.WorkspacePath = v
	}
	if enabledSkills := data["enabledSkills"]; enabledSkills != nil {
		s.EnabledSkills = JSONField{Data: enabledSkills}
	}
	if v, ok := data["parentID"].(string); ok {
		s.ParentID = v
	}
	if v, ok := data["title"].(string); ok {
		s.Title = v
	}
	if v, ok := data["version"].(string); ok {
		s.Version = v
	}

	if time, ok := data["time"].(map[string]interface{}); ok {
		if v, ok := time["created"].(int64); ok {
			s.Created = v
		}
		if v, ok := time["updated"].(int64); ok {
			s.Updated = v
		}
		if v, ok := time["compacting"].(int64); ok {
			s.Compacting = v
		}
		if v, ok := time["archived"].(int64); ok {
			s.Archived = v
		}
	}

	if summary := data["summary"]; summary != nil {
		s.Summary = JSONField{Data: summary}
	}
	if share := data["share"]; share != nil {
		s.Share = JSONField{Data: share}
	}
	if perm := data["permission"]; perm != nil {
		s.Permission = JSONField{Data: perm}
	}
	if revert := data["revert"]; revert != nil {
		s.Revert = JSONField{Data: revert}
	}
	if tokens := data["tokens"]; tokens != nil {
		s.Tokens = JSONField{Data: tokens}
	}

	return s
}

// JSONField 用于存储 JSON 数据的字段类型
type JSONField struct {
	Data interface{}
}

// Scan 实现 sql.Scanner 接口
func (j *JSONField) Scan(value interface{}) error {
	if value == nil {
		j.Data = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	if len(bytes) == 0 {
		j.Data = nil
		return nil
	}

	var data interface{}
	if err := json.Unmarshal(bytes, &data); err != nil {
		return err
	}

	j.Data = data
	return nil
}

// Value 实现 driver.Valuer 接口
func (j JSONField) Value() (driver.Value, error) {
	if j.Data == nil {
		return nil, nil
	}

	bytes, err := json.Marshal(j.Data)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// PermissionFromRuleset 从 permission.Ruleset 创建 JSONField
func PermissionFromRuleset(ruleset permission.Ruleset) JSONField {
	return JSONField{Data: ruleset}
}
