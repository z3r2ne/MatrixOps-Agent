package models

// Message 消息数据库模型
type Message struct {
	ID         string `gorm:"primaryKey;size:255"`
	SessionID  string `gorm:"size:255;index;not null"`
	Role         string `gorm:"size:50;index"`
	MessageKind  string `gorm:"size:50;index"`
	MessageOrigin string `gorm:"size:100"`
	ParentID   string `gorm:"size:255;index"`
	Mode       string `gorm:"size:100"`
	Agent      string `gorm:"size:255"`
	Name       string `gorm:"size:255"`
	Occupation string `gorm:"size:255"`
	ProviderID string `gorm:"size:255"`
	ModelID    string `gorm:"size:255"`
	System     string `gorm:"type:text"`
	Variant    string `gorm:"size:100"`
	Finish     string `gorm:"size:100"`
	Cost       float64
	State      string    `gorm:"size:100"`
	Snapshot   string    `gorm:"size:255"`
	Phase      string    `gorm:"size:100"`
	ResponsesOutputMessageRaw  string `gorm:"type:text"`
	ResponsesReasoningItemRawsJSON string `gorm:"type:text"`
	Memory     JSONField `gorm:"type:text"`

	// 时间字段
	Created   int64 `gorm:"index"`
	Completed int64

	// JSON 字段
	Model   JSONField `gorm:"type:text"`
	Tools   JSONField `gorm:"type:text"`
	Summary JSONField `gorm:"type:text"`
	Tokens  JSONField `gorm:"type:text"`
	Error   JSONField `gorm:"type:text"`
	Path    JSONField `gorm:"type:text"`
}

// TableName 指定表名
func (Message) TableName() string {
	return "messages"
}

// Part 消息部件数据库模型
type Part struct {
	ID          string `gorm:"primaryKey;size:255"`
	MessageID   string `gorm:"size:255;index;not null"`
	SessionID   string `gorm:"size:255;index;not null"`
	Type        string `gorm:"size:100;index"`
	Text        string `gorm:"type:text"`
	Reasoning   string `gorm:"type:text"`
	Synthetic   bool
	Ignored     bool
	Snapshot    string `gorm:"size:255"`
	Hash        string `gorm:"size:255"`
	Mime        string `gorm:"size:255"`
	Filename    string `gorm:"type:text"`
	URL         string `gorm:"type:text"`
	AgentName   string `gorm:"size:255"`
	Auto        bool
	Description string `gorm:"type:text"`
	Subagent    string `gorm:"size:255"`
	Command     string `gorm:"type:text"`
	Attempt     int
	Reason      string `gorm:"type:text"`
	Cost        float64

	// 时间字段
	TimeStart     int64
	TimeEnd       int64
	TimeCreated   int64
	TimeCompacted int64

	// JSON 字段
	Tool     JSONField `gorm:"type:text"`
	Metadata JSONField `gorm:"type:text"`
	Files    JSONField `gorm:"type:text"`
	Source   JSONField `gorm:"type:text"`
	Model    JSONField `gorm:"type:text"`
	Error    JSONField `gorm:"type:text"`
	Tokens   JSONField `gorm:"type:text"`
}

// TableName 指定表名
func (Part) TableName() string {
	return "parts"
}
