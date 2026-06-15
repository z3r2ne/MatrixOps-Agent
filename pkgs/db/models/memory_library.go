package models

import "time"

// MemoryLibrary 项目可关联的背景记忆库（名称 + 介绍）；RAG 知识库用于语义检索。
type MemoryLibrary struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"not null;uniqueIndex"`
	Content     string    `json:"content" gorm:"type:text"` // 记忆库：介绍；RAG：知识内容
	IsRag       bool      `json:"isRag" gorm:"index;not null;default:false"`
	IsTemporary bool      `json:"isTemporary" gorm:"index;not null;default:false"`
	TaskID      *uint     `json:"taskId,omitempty" gorm:"index"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type MemoryLibraryCreate struct {
	Name        string `json:"name" binding:"required"`
	Content     string `json:"content"`
	IsRag       bool   `json:"isRag"`
	IsTemporary bool   `json:"isTemporary"`
	TaskID      *uint  `json:"taskId"`
}

type MemoryLibraryUpdate struct {
	Name    *string `json:"name"`
	Content *string `json:"content"`
}

type MemoryLibraryPromoteRequest struct {
	Name *string `json:"name"`
}
