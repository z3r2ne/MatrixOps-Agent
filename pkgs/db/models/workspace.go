package models

import (
	"time"
)

// Workspace 工作区模型
type Workspace struct {
	ID         uint              `json:"id" gorm:"primaryKey"`
	Type       WorkspaceType     `json:"type" gorm:"type:text;not null;default:'code'"`
	Name       string            `json:"name" gorm:"not null"`
	Path       string            `json:"path" gorm:"not null;unique"`
	Icon       string            `json:"icon" gorm:"default:'folder'"`
	Color      string            `json:"color" gorm:"default:'blue'"`
	GroupMode  TaskListGroupMode `json:"groupMode" gorm:"type:text;not null;default:'project'"`
	Active     bool              `json:"active" gorm:"default:false"`
	ProjectIDs []uint            `json:"projectIds" gorm:"type:json;serializer:json"` // 关联的项目ID列表
	Projects   []Project         `json:"projects,omitempty" gorm:"-"`                 // 用于查询时填充，不存储
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
}

// WorkspaceCreate 创建工作区请求
type WorkspaceCreate struct {
	Type      string          `json:"type"` // 可选，默认 code
	Name      string          `json:"name" binding:"required"`
	Path      string          `json:"path"` // 可选，如果为空则自动生成
	Icon      string          `json:"icon"`
	Color     string          `json:"color"`
	GroupMode *string         `json:"groupMode"`
	Projects  []ProjectCreate `json:"projects"` // 关联的项目列表
}

// WorkspaceUpdate 更新工作区请求
type WorkspaceUpdate struct {
	Type      *string `json:"type"`
	Name      *string `json:"name"`
	Path      *string `json:"path"`
	Icon      *string `json:"icon"`
	Color     *string `json:"color"`
	GroupMode *string `json:"groupMode"`
	Active    *bool   `json:"active"`
}

// WorkspaceResponse 工作区响应（包含状态）
type WorkspaceResponse struct {
	Workspace
	PathExists bool   `json:"pathExists"`
	Error      string `json:"error,omitempty"`
}
