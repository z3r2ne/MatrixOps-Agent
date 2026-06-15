package models

// MemoryEntry 会话记忆数据库模型
type MemoryEntry struct {
	ID                             uint   `gorm:"primaryKey"`
	SessionID                      string `gorm:"size:255;index;not null"`
	SourceMessageID                string `gorm:"size:255;index"`
	SourcePartID                   string `gorm:"size:255;index"`
	EntryKind                      string `gorm:"size:100;index;not null"`
	Role                           string `gorm:"size:50;index;not null"`
	Content                        string `gorm:"type:text"`
	RawOutput                      string `gorm:"type:text"`
	Phase                          string `gorm:"size:100;index"`
	ResponsesOutputMessageRaw      string `gorm:"type:text"`
	ResponsesReasoningItemRawsJSON string `gorm:"type:text"`
	ReasoningContent               string `gorm:"type:text"`
	ThinkingSignature              string `gorm:"type:text"`
	CallToolInfo                   string `gorm:"type:text"`
	ToolCallID                     string `gorm:"size:255;index"`
	ToolName                       string `gorm:"size:255;index"`
	ToolStatus                     string `gorm:"size:100;index"`
	ToolReason                     string `gorm:"type:text"`
	ToolRequestRawJSON             string `gorm:"type:text"`
	ToolInputJSON                  string `gorm:"type:text"`
	ToolOutput                     string `gorm:"type:text"`
	ToolSystemMessage              string `gorm:"type:text"`
	ToolError                      string `gorm:"type:text"`
	ToolTitle                      string `gorm:"type:text"`
	ToolMetadataJSON               string `gorm:"type:text"`
	Synthetic                      bool `gorm:"index;default:false"`
	SearchArchived                 bool `gorm:"index;default:false"`
	CompressionLevel               int  `gorm:"index;default:0"`
	Sequence                       int64 `gorm:"index;not null"`
	TokenCount                     int
	Created                        int64 `gorm:"index"`
	Updated                        int64 `gorm:"index"`
}

func (MemoryEntry) TableName() string {
	return "memory_entries"
}
