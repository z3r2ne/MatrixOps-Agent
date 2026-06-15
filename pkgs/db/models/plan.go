package models

// Plan 计划数据库模型
type Plan struct {
	ID        uint      `gorm:"primaryKey"`
	SessionID string    `gorm:"size:255;uniqueIndex;not null"`
	Content   JSONField `gorm:"type:text"`
	Created   int64     `gorm:"index"`
	Updated   int64     `gorm:"index"`
}

// TableName 指定表名
func (Plan) TableName() string {
	return "plans"
}
