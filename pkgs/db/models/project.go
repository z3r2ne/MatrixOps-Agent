package models

import (
	"strconv"
	"time"
)

const (
	ProjectToolPermissionAllow = "allow"
	ProjectToolPermissionAsk   = "ask"
	ProjectToolPermissionDeny  = "deny"
)

// Project 项目模型
type Project struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	Name string `json:"name" gorm:"not null"`
	Path string `json:"path" gorm:"not null"`
	// WorktreePath 为项目辅助目录根（例如 ai_workspace 的默认根）；任务分支 worktree 统一创建到全局数据目录。
	WorktreePath     string          `json:"worktreePath" gorm:"not null;default:''"`
	Icon             string          `json:"icon" gorm:"default:'code'"`
	Color            string          `json:"color" gorm:"default:'blue'"`
	Status           string          `json:"status" gorm:"default:'Idle'"` // Active, Idle
	ActiveTasks      int             `json:"activeTasks" gorm:"default:0"`
	Prompt           string          `json:"prompt" gorm:"type:text"` // 项目专属提示词
	ToolPermissions  string          `json:"toolPermissions" gorm:"type:text"`
	MemoryLibraryIDs UintSlice       `json:"memoryLibraryIds" gorm:"type:json;serializer:json"`
	MemoryLibraries  []MemoryLibrary `json:"memoryLibraries,omitempty" gorm:"-"`
	YoloMode         bool            `json:"yoloMode" gorm:"default:false"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

func (p *Project) GetID() string {
	return strconv.FormatUint(uint64(p.ID), 10)
}

func ConvertProjectIDToString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

// ProjectCreate 创建项目请求
type ProjectCreate struct {
	Name             string `json:"name" binding:"required"`
	Path             string `json:"path"`      // 可选，用于已存在的项目
	CreateNew        bool   `json:"createNew"` // 是否创建新项目
	NewPath          string `json:"newPath"`   // 创建新项目时的父目录路径
	Icon             string `json:"icon"`
	Color            string `json:"color"`
	ToolPermissions  string `json:"toolPermissions"`
	MemoryLibraryIDs []uint `json:"memoryLibraryIds"`
	YoloMode         bool   `json:"yoloMode"`
}

// ProjectUpdate 更新项目请求
type ProjectUpdate struct {
	Name             *string `json:"name"`
	Path             *string `json:"path"`
	Icon             *string `json:"icon"`
	Color            *string `json:"color"`
	Status           *string `json:"status"`
	ActiveTasks      *int    `json:"activeTasks"`
	Prompt           *string `json:"prompt"`
	ToolPermissions  *string `json:"toolPermissions"`
	MemoryLibraryIDs *[]uint `json:"memoryLibraryIds"`
	YoloMode         *bool   `json:"yoloMode"`
}

// ProjectResponse 项目响应（包含状态）
type ProjectResponse struct {
	Project
	PathExists bool   `json:"pathExists"`
	Error      string `json:"error,omitempty"`
}
