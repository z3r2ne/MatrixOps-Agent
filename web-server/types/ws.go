package types

import "pkgs/db/models"

type WSMessageType string

type WSOutgoingMessage struct {
	Type      WSMessageType        `json:"type"`
	TaskID    uint                 `json:"taskId,omitempty"`
	Message   *models.TaskMessage  `json:"message,omitempty"`
	Status    string               `json:"status,omitempty"`
	Error     string               `json:"error,omitempty"`
	SessionID string               `json:"sessionId,omitempty"`
	History   []models.TaskMessage `json:"history,omitempty"`
	Data      any                  `json:"data,omitempty"`
}

const (
	WSTypeTaskMessage   WSMessageType = "task_message"    // 任务消息
	WSTypeMessageV2     WSMessageType = "message_v2"      // 消息 v2
	WSTypeTaskStatus    WSMessageType = "task_status"     // 任务状态变化
	WSTypeError         WSMessageType = "error"           // 错误
	WSTypeSessionTitle  WSMessageType = "session_title"   // 会话标题
	WSTypeSubscribed    WSMessageType = "subscribed"      // 订阅成功
	WSTypeUnsubscribed  WSMessageType = "unsubscribed"    // 取消订阅成功
	WSTypeHistory       WSMessageType = "history"         // 历史消息
	WSTypeRetry         WSMessageType = "retry"           // 重试
	WSTypeIsWorking     WSMessageType = "is_working"      // 开始 shimmer
	WSTypeIsNotWorking  WSMessageType = "is_not_working"  // 结束 shimmer
	WSTypeEnd           WSMessageType = "end"             // 结束
	WSTypeWaitUserInput WSMessageType = "wait_user_input" // 等待用户输入
	WSTypeTaskQueue            WSMessageType = "task_queue"             // 任务消息队列变更
	WSTypeTaskPlan             WSMessageType = "task_plan"              // 任务计划变更
	WSTypeILinkSessionExpired  WSMessageType = "ilink_session_expired"  // iLink 微信会话过期，需重新扫码登录

)
