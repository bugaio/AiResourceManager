// Package handler ws.go 提供 WebSocket 连接的 HTTP 升级和消息处理
package handler

import (
	"net/http"
	"time"

	"github.com/anthropic/airesourcemanager/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 允许所有来源的 WebSocket 连接（开发环境）
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSHandler WebSocket 处理器，负责连接升级和生命周期管理
type WSHandler struct {
	hub    *service.Hub
	logger *zap.Logger
}

// NewWSHandler 创建 WebSocket 处理器实例
// 参数 hub: WebSocket Hub 用于管理连接
// 参数 logger: 日志实例
// 返回: WSHandler 指针
func NewWSHandler(hub *service.Hub, logger *zap.Logger) *WSHandler {
	return &WSHandler{
		hub:    hub,
		logger: logger,
	}
}

// HandleWS 处理 WebSocket 升级请求
// 将 HTTP 连接升级为 WebSocket，注册到 Hub，并启动读写协程
func (h *WSHandler) HandleWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("WebSocket 升级失败", zap.Error(err))
		return
	}

	client := service.NewClient(h.hub, conn, h.logger)
	h.hub.Register(client)

	// 启动读写协程
	go client.WritePump()
	go client.ReadPump()
}

const (
	// writeWait 写入超时时间
	WriteWait = 10 * time.Second
	// pongWait 等待 pong 响应的超时时间
	PongWait = 60 * time.Second
	// pingPeriod 发送 ping 的间隔，必须小于 pongWait
	PingPeriod = 30 * time.Second
	// maxMessageSize 最大消息大小
	MaxMessageSize = 512 * 1024
)
