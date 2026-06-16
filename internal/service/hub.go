// Package service hub.go 实现 WebSocket 连接的集中管理
// 使用 goroutine + channel 模式处理连接的注册、注销和广播
package service

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 30 * time.Second
	maxMsgSize = 512 * 1024
)

// Hub WebSocket 连接管理中心
// 维护所有活跃连接，处理注册、注销和消息广播
type Hub struct {
	clients    sync.Map         // 存储所有活跃的客户端连接
	register   chan *Client     // 注册通道
	unregister chan *Client     // 注销通道
	broadcast  chan []byte      // 广播消息通道
	done       chan struct{}    // 关闭信号
	logger     *zap.Logger
}

// Client WebSocket 客户端连接封装
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte // 待发送消息缓冲
	logger *zap.Logger
}

// NewHub 创建 Hub 实例
// 参数 logger: 日志实例
// 返回: 初始化完成的 Hub 指针
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
		done:       make(chan struct{}),
		logger:     logger,
	}
}

// NewClient 创建客户端连接实例
// 参数 hub: 所属的 Hub
// 参数 conn: WebSocket 连接
// 参数 logger: 日志实例
// 返回: 初始化完成的 Client 指针
func NewClient(hub *Hub, conn *websocket.Conn, logger *zap.Logger) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		logger: logger,
	}
}

// Run 启动 Hub 主循环，处理注册、注销和广播事件
// 应在独立 goroutine 中运行
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients.Store(client, true)
			h.logger.Info("客户端已连接")
		case client := <-h.unregister:
			if _, ok := h.clients.Load(client); ok {
				h.clients.Delete(client)
				close(client.send)
				h.logger.Info("客户端已断开")
			}
		case message := <-h.broadcast:
			h.clients.Range(func(key, value interface{}) bool {
				client := key.(*Client)
				select {
				case client.send <- message:
				default:
					// 发送缓冲满，断开连接
					h.clients.Delete(client)
					close(client.send)
				}
				return true
			})
		case <-h.done:
			// 关闭所有连接
			h.clients.Range(func(key, value interface{}) bool {
				client := key.(*Client)
				close(client.send)
				h.clients.Delete(client)
				return true
			})
			return
		}
	}
}

// Register 注册新客户端到 Hub
// 参数 client: 要注册的客户端
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister 从 Hub 注销客户端
// 参数 client: 要注销的客户端
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Broadcast 向所有连接的客户端广播消息
// 参数 message: 要广播的消息字节
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// Shutdown 关闭 Hub，断开所有连接
func (h *Hub) Shutdown() {
	close(h.done)
}

// CloseAll 关闭所有活跃的 WebSocket 连接并清理客户端列表
func (h *Hub) CloseAll() {
	h.clients.Range(func(key, value interface{}) bool {
		client := key.(*Client)
		client.conn.Close()
		h.clients.Delete(key)
		return true
	})
}

// ReadPump 客户端读取协程，处理接收到的消息
// 当连接断开或出错时自动注销客户端
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMsgSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Error("WebSocket 读取异常", zap.Error(err))
			}
			break
		}
	}
}

// WritePump 客户端写入协程，负责发送消息和心跳 ping
// 每 30 秒发送一次 ping，超过 60 秒未收到 pong 则断开连接
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub 关闭了 send channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
