package models

import "time"

const DefaultMemorySearchTopK = 8

// MemorySearchDocument 记忆检索索引元数据（向量存 chromem-go，全文存 bleve）。
type MemorySearchDocument struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	SessionID       string    `json:"sessionId" gorm:"index"`
	MemoryEntryID   uint      `json:"memoryEntryId" gorm:"index"`
	MemoryLibraryID uint      `json:"memoryLibraryId" gorm:"index"`
	SourceKind      string    `json:"sourceKind" gorm:"index;not null"`
	Content         string    `json:"content" gorm:"type:text;not null"`
	ContentHash     string    `json:"contentHash" gorm:"index;not null"`
	Dimension       int       `json:"dimension"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (MemorySearchDocument) TableName() string {
	return "memory_search_documents"
}
