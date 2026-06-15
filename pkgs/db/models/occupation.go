package models

import "time"

// Occupation 职业配置
type Occupation struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Code        string    `json:"code" gorm:"unique;not null"` // 职业代码，如: coder, analyst
	Name        string    `json:"name" gorm:"not null"`        // 职业名称，如: 研发工程师, 分析师
	Description string    `json:"description"`                 // 职业描述
	Prompt      string    `json:"prompt" gorm:"type:text"`     // 职业专属提示词
	Color       string    `json:"color"`                       // 职业显示颜色
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// OccupationCreate 创建职业
type OccupationCreate struct {
	Code        string `json:"code" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Color       string `json:"color"`
}

// OccupationUpdate 更新职业
type OccupationUpdate struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Prompt      *string `json:"prompt"`
	Color       *string `json:"color"`
}
