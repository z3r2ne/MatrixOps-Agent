package models

import "time"

const (
	McpTransportStdio = "stdio"
	McpTransportSSE   = "sse"
	McpTransportHTTP  = "http"
)

type McpServer struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	Name             string    `json:"name" gorm:"not null;uniqueIndex"`
	Transport        string    `json:"transport" gorm:"not null;default:'stdio'"`
	Command          string    `json:"command" gorm:"type:text"`
	ArgsJSON         string    `json:"argsJson" gorm:"type:text;not null;default:'[]'"`
	EnvJSON          string    `json:"envJson" gorm:"type:text;not null;default:'{}'"`
	URL              string    `json:"url" gorm:"type:text"`
	HeadersJSON      string    `json:"headersJson" gorm:"type:text;not null;default:'{}'"`
	Enabled          bool      `json:"enabled" gorm:"not null;default:true"`
	ToolCount        int       `json:"toolCount" gorm:"not null;default:0"`
	Connected        bool      `json:"connected" gorm:"not null;default:false"`
	LastConnectError string    `json:"lastConnectError,omitempty" gorm:"type:text"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type McpServerCreate struct {
	Name      string `json:"name" binding:"required"`
	Transport string `json:"transport"`
	Command   string `json:"command"`
	ArgsJSON  string `json:"argsJson"`
	EnvJSON   string `json:"envJson"`
	URL         string `json:"url"`
	HeadersJSON string `json:"headersJson"`
	Enabled     *bool  `json:"enabled"`
}

type McpServerUpdate struct {
	Name      *string `json:"name"`
	Transport *string `json:"transport"`
	Command   *string `json:"command"`
	ArgsJSON  *string `json:"argsJson"`
	EnvJSON   *string `json:"envJson"`
	URL         *string `json:"url"`
	HeadersJSON *string `json:"headersJson"`
	Enabled     *bool   `json:"enabled"`
}

type McpToolInfo struct {
	Name        string                 `json:"name"`
	FullName    string                 `json:"fullName"`
	ServerName  string                 `json:"serverName"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}
