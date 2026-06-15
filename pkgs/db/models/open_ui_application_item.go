package models

import "time"

const (
	OpenUIKindWorkspace = "workspace"
	OpenUIKindProject   = "project"
)

// OpenUIApplicationItem 记录桌面端「已打开」的工作区或项目（顺序按主键递增）。
type OpenUIApplicationItem struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Kind       string    `json:"kind" gorm:"size:32;not null;uniqueIndex:ux_open_ui_kind_resource"`
	ResourceID uint      `json:"resourceId" gorm:"not null;column:resource_id;uniqueIndex:ux_open_ui_kind_resource"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (OpenUIApplicationItem) TableName() string {
	return "open_ui_application_items"
}
