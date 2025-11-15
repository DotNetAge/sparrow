package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Client 表示一个 WebSocket 客户端连接
type Client struct {
	ID     string          // 客户端唯一标识
	Conn   *websocket.Conn // WebSocket 连接
	UserID string          // 用户ID（可选，用于业务层关联）
}

// 全局变量：升级器 + 线程安全连接池
var (
	upgrader = websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// 简化连接池：只使用一个 map，通过 clientID 索引
	clients = struct {
		sync.RWMutex
		data map[string]*Client
	}{
		data: make(map[string]*Client),
	}
)

/*
WebSocket 机制使用指南

详细文档：docs/WebSocket机制使用指南.md

核心功能：
- BroadcastText/BroadcastJSON: 全局广播
- SendTextToClient/SendJSONToClient: 单点发送
- SendTextToUser/SendJSONToUser: 用户广播
- GetOnlineCount: 在线统计

快速开始：
1. r.Use(ws.Middleware())
2. r.GET("/ws", func(c *gin.Context) {})
3. ws.BroadcastText("Hello World")
4. ws.GetOnlineCount()

示例：
	// 广播API
	r.POST("/api/broadcast", func(c *gin.Context) {
		var req struct { Message string `json:"message" binding:"required"` }
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request"})
			return
		}

		if err := ws.BroadcastText(req.Message); err != nil {
			c.JSON(500, gin.H{"error": "Broadcast failed"})
			return
		}
		c.JSON(200, gin.H{"status": "success"})
	})

	// 定时统计广播
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			stats := map[string]interface{}{
				"type": "system_stats",
				"online_count": ws.GetOnlineCount(),
				"server_time": time.Now().Format("2006-01-02 15:04:05"),
			}
			if err := ws.BroadcastJSON(stats); err != nil {
				log.Printf("统计广播失败: %v", err)
			}
		}
	}()
*/

// 配置选项：支持自定义跨域校验、心跳间隔等（可选）
// 配置选项
type Option func()

// WithCheckOrigin：自定义跨域校验规则
func WithCheckOrigin(check func(r *http.Request) bool) Option {
	return func() {
		upgrader.CheckOrigin = check
	}
}

// WithHeartbeatInterval：自定义心跳间隔（默认10秒）
func WithHeartbeatInterval(interval time.Duration) Option {
	return func() {
		heartbeatInterval = interval
	}
}

var heartbeatInterval = 10 * time.Second

// Middleware：Gin 中间件函数（核心入口）
// 用法：r.Use(ws.Middleware(ws.WithCheckOrigin(...)))
// Middleware：Gin 中间件函数
func Middleware(opts ...Option) gin.HandlerFunc {
	// 应用配置
	for _, opt := range opts {
		opt()
	}

	return func(c *gin.Context) {
		// 非WebSocket请求直接放行
		if !isWebSocketHandshake(c.Request) {
			c.Next()
			return
		}

		// 升级WebSocket连接
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("[WebSocket] 连接升级失败：%v", err)
			c.AbortWithStatusJSON(500, gin.H{"error": "WebSocket 连接失败"})
			return
		}

		// 生成客户端ID
		clientID := generateClientID(c)
		userID := c.Query("user_id")
		if userID == "" {
			userID = c.GetHeader("X-User-ID")
		}

		// 创建客户端
		client := &Client{
			ID:     clientID,
			Conn:   conn,
			UserID: userID,
		}

		addClient(client)
		log.Printf("[WebSocket] 客户端连接：%s，在线数：%d", clientID, len(clients.data))

		// 启动消息处理和心跳
		go readMsg(client)
		go startHeartbeat(client)

		c.Abort()
	}
}

// Broadcast：广播消息给所有客户端（保持向后兼容）
func Broadcast(messageType int, data []byte) error {
	clients.RLock()
	clientList := make([]*Client, 0, len(clients.data))
	for _, client := range clients.data {
		clientList = append(clientList, client)
	}
	clients.RUnlock()

	var errs []error
	var failedClients []string

	for _, client := range clientList {
		if err := client.Conn.WriteMessage(messageType, data); err != nil {
			errs = append(errs, err)
			failedClients = append(failedClients, client.ID)
		}
	}

	// 移除发送失败的客户端
	for _, clientID := range failedClients {
		removeClient(clientID)
		log.Printf("[WebSocket] 广播失败，移除客户端 %s", clientID)
	}

	if len(errs) > 0 {
		return fmt.Errorf("部分客户端发送失败: %v", errs)
	}
	return nil
}

// BroadcastText 使用便捷接口发送文本广播
func BroadcastText(text string) error {
	return Broadcast(websocket.TextMessage, []byte(text))
}

// BroadcastJSON 使用便捷接口发送JSON广播
func BroadcastJSON(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %w", err)
	}
	return Broadcast(websocket.TextMessage, jsonData)
}

// SendToClient：单点发送函数（保持向后兼容）
func SendToClient(clientID string, msgType int, data []byte) bool {
	clients.RLock()
	client, exists := clients.data[clientID]
	clients.RUnlock()

	if !exists {
		return false
	}

	if err := client.Conn.WriteMessage(msgType, data); err != nil {
		log.Printf("[WebSocket] 发送失败，移除客户端 %s：%v", clientID, err)
		removeClient(clientID)
		return false
	}
	return true
}

// SendTextToClient 使用便捷接口发送文本消息给指定客户端
func SendTextToClient(clientID string, text string) error {
	if !SendToClient(clientID, websocket.TextMessage, []byte(text)) {
		return fmt.Errorf("客户端 %s 不在线或发送失败", clientID)
	}
	return nil
}

// SendJSONToClient 使用便捷接口发送JSON消息给指定客户端
func SendJSONToClient(clientID string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %w", err)
	}
	if !SendToClient(clientID, websocket.TextMessage, jsonData) {
		return fmt.Errorf("客户端 %s 不在线或发送失败", clientID)
	}
	return nil
}

// SendBinaryToClient 使用便捷接口发送二进制数据给指定客户端
func SendBinaryToClient(clientID string, data []byte) error {
	if !SendToClient(clientID, websocket.BinaryMessage, data) {
		return fmt.Errorf("客户端 %s 不在线或发送失败", clientID)
	}
	return nil
}

// SendToUser：发送消息给指定用户的所有客户端
func SendToUser(userID string, msgType int, data []byte) error {
	clients.RLock()
	var clientList []*Client
	for _, client := range clients.data {
		if client.UserID == userID {
			clientList = append(clientList, client)
		}
	}
	clients.RUnlock()

	if len(clientList) == 0 {
		return fmt.Errorf("用户 %s 没有在线客户端", userID)
	}

	var errs []error
	var failedClients []string

	for _, client := range clientList {
		if err := client.Conn.WriteMessage(msgType, data); err != nil {
			errs = append(errs, err)
			failedClients = append(failedClients, client.ID)
		}
	}

	// 移除发送失败的客户端
	for _, clientID := range failedClients {
		removeClient(clientID)
		log.Printf("[WebSocket] 用户消息发送失败，移除客户端 %s", clientID)
	}

	if len(errs) > 0 {
		return fmt.Errorf("部分客户端发送失败: %v", errs)
	}
	return nil
}

// SendTextToUser 使用便捷接口发送文本消息给指定用户的所有客户端
func SendTextToUser(userID string, text string) error {
	return SendToUser(userID, websocket.TextMessage, []byte(text))
}

// SendJSONToUser 使用便捷接口发送JSON消息给指定用户的所有客户端
func SendJSONToUser(userID string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %w", err)
	}
	return SendToUser(userID, websocket.TextMessage, jsonData)
}

// SendBinaryToUser 使用便捷接口发送二进制数据给指定用户的所有客户端
func SendBinaryToUser(userID string, data []byte) error {
	return SendToUser(userID, websocket.BinaryMessage, data)
}

// GetOnlineCount：获取在线客户端数量
func GetOnlineCount() int {
	clients.RLock()
	defer clients.RUnlock()
	return len(clients.data)
}

// 辅助函数
func isWebSocketHandshake(req *http.Request) bool {
	return req.Header.Get("Upgrade") == "websocket" && req.Header.Get("Connection") == "Upgrade"
}

func generateClientID(c *gin.Context) string {
	if clientID := c.Query("client_id"); clientID != "" {
		return clientID
	}
	if clientID := c.GetHeader("X-Client-ID"); clientID != "" {
		return clientID
	}
	return fmt.Sprintf("client_%d", time.Now().UnixNano())
}

func addClient(client *Client) {
	clients.Lock()
	clients.data[client.ID] = client
	clients.Unlock()
}

func removeClient(clientID string) {
	clients.Lock()
	defer clients.Unlock()

	if client, exists := clients.data[clientID]; exists {
		delete(clients.data, clientID)
		_ = client.Conn.Close()
	}
}

// 辅助函数：读取客户端消息（异常时移除连接）
func readMsg(client *Client) {
	defer func() {
		removeClient(client.ID)
		log.Printf("[WebSocket] 客户端断开：%s，在线数：%d", client.ID, len(clients.data))
	}()

	for {
		messageType, data, err := client.Conn.ReadMessage()
		if err != nil {
			log.Printf("[WebSocket] 读取消息失败：%v", err)
			break
		}

		switch messageType {
		case websocket.TextMessage:
			log.Printf("[WebSocket] 收到文本消息：%s", string(data))
		case websocket.BinaryMessage:
			log.Printf("[WebSocket] 收到二进制消息：%d bytes", len(data))
		}

		// 业务层根据需要调用 Broadcast、SendToClient、SendToUser 等函数
	}
}

// 辅助函数：心跳检测（避免连接被网关断开）
func startHeartbeat(client *Client) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := client.Conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
			log.Printf("[WebSocket] 心跳失败：%v", err)
			removeClient(client.ID)
			break
		}
	}
}
