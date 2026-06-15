package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"matrixops/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

var clientCounter uint64

var globalUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// GlobalWSHandler 全局 WebSocket 处理器
type GlobalWSHandler struct {
	hub *services.GlobalWSHub
}

// NewGlobalWSHandler 创建全局 WebSocket 处理器
func NewGlobalWSHandler(db *gorm.DB) *GlobalWSHandler {
	hub := services.GetGlobalWSHub(db)
	return &GlobalWSHandler{
		hub: hub,
	}
}

// Handle 处理 WebSocket 连接
func (h *GlobalWSHandler) Handle(c *gin.Context) {
	conn, err := globalUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[GlobalWS] Failed to upgrade: %v", err)
		return
	}
	defer conn.Close()

	// 创建客户端
	clientID := fmt.Sprintf("c%d-%d", atomic.AddUint64(&clientCounter, 1), time.Now().UnixNano()%10000)
	client := services.NewGlobalWSClient(clientID)

	// 注册到 Hub
	h.hub.Register(client)
	defer h.hub.Unregister(client)

	// 启动写入 goroutine
	go func() {
		for message := range client.Send {
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("[GlobalWS] Write error: %v", err)
				return
			}
		}
	}()

	// 读取消息循环
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[GlobalWS] Read error: %v", err)
			}
			break
		}

		var incoming services.WSIncomingMessage
		_ = json.Unmarshal(message, &incoming)

		err = h.hub.HandleMessage(client, message)
		if err != nil && incoming.TaskID != 0 {
			h.hub.BroadcastError(incoming.TaskID, err.Error())
		}
	}
}
