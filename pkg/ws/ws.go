package ws

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// 使用示例
// func main() {
// 	// 1. 初始化 Gin 引擎
// 	r := gin.Default()

// 	// 2. 载入 WebSocket 中间件（支持自定义配置，如跨域、心跳）
// 	r.Use(ws.Middleware(
// 		// 可选配置：生产环境指定允许的跨域域名（示例：允许前端域名 http://localhost:3000）
// 		ws.WithCheckOrigin(func(r *http.Request) bool {
// 			origin := r.Header.Get("Origin")
// 			return origin == "http://localhost:3000" || origin == ""
// 		}),
// 		// 可选配置：自定义心跳间隔为 15 秒
// 		ws.WithHeartbeatInterval(15*time.Second),
// 	))

// 	// 3. 注册 WebSocket 路由（中间件会自动处理握手和连接）
// 	// 注意：路由路径可自定义（如 /ws），中间件会通过请求头判断是否为 WebSocket 握手
// 	r.GET("/ws", func(c *gin.Context) {
// 		// 这里可以留空，中间件已处理所有逻辑
// 		// 若需要在连接建立时做额外操作（如绑定用户 ID），可在这里添加
// 		log.Println("[WebSocket] 连接建立成功")
// 	})

// 	// 示例 1：API 接口触发广播（如管理员发送通知）
// 	r.POST("/api/broadcast", func(c *gin.Context) {
// 		var req struct {
// 			Msg string `json:"msg" binding:"required"`
// 		}
// 		if err := c.ShouldBindJSON(&req); err != nil {
// 			c.JSON(400, gin.H{"error": "请输入消息内容"})
// 			return
// 		}

// 		// 调用 ws 包的全局 Broadcast 函数，广播管理员通知
// 		ws.Broadcast(websocket.TextMessage, []byte("管理员通知："+req.Msg))
// 		c.JSON(200, gin.H{"status": "success", "msg": "广播已发送"})
// 	})

// 	// 示例 2：外部 Goroutine 触发广播（如定时任务）
// 	go func() {
// 		ticker := time.NewTicker(20 * time.Second)
// 		defer ticker.Stop()
// 		for range ticker.C {
// 			systemMsg := "系统通知：" + time.Now().Format("2006-01-02 15:04:05") + " - 服务正常运行"
// 			ws.Broadcast(websocket.TextMessage, []byte(systemMsg))
// 		}
// 	}()

// 	// 启动服务
// 	log.Println("服务启动：http://localhost:8080，WebSocket 地址：ws://localhost:8080/ws")
// 	if err := r.Run(":8080"); err != nil {
// 		log.Fatalf("服务启动失败：%v", err)
// 	}
// }

// 全局变量：升级器 + 线程安全连接池（中间件初始化后自动可用）
var (
	upgrader = websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		// 跨域配置：可通过 WithCheckOrigin 函数自定义，默认允许所有
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	clientPool = struct {
		sync.RWMutex
		conns map[*websocket.Conn]bool
	}{
		conns: make(map[*websocket.Conn]bool),
	}
)

// 配置选项：支持自定义跨域校验、心跳间隔等（可选）
type Option func()

// WithCheckOrigin：自定义跨域校验规则（如生产环境指定允许的域名）
func WithCheckOrigin(check func(r *http.Request) bool) Option {
	return func() {
		upgrader.CheckOrigin = check
	}
}

// WithHeartbeatInterval：自定义心跳间隔（默认 10 秒）
func WithHeartbeatInterval(interval time.Duration) Option {
	return func() {
		heartbeatInterval = interval
	}
}

var heartbeatInterval = 10 * time.Second // 默认心跳间隔

// Middleware：Gin 中间件函数（核心入口）
// 用法：r.Use(ws.Middleware(ws.WithCheckOrigin(...)))
func Middleware(opts ...Option) gin.HandlerFunc {
	// 应用自定义配置（如跨域、心跳）
	for _, opt := range opts {
		opt()
	}

	// 返回 Gin 中间件 HandlerFunc
	return func(c *gin.Context) {
		// 仅处理 WebSocket 路由（通过路径匹配，可根据需求调整）
		// 若当前请求不是 WebSocket 握手，直接放行给后续路由
		if !isWebSocketHandshake(c.Request) {
			c.Next()
			return
		}

		// 1. 升级 HTTP 连接为 WebSocket 连接
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("[WebSocket] 连接升级失败：%v", err)
			c.AbortWithStatusJSON(500, gin.H{"error": "WebSocket 连接失败"})
			return
		}

		// 2. 新连接加入连接池
		clientPool.Lock()
		clientPool.conns[conn] = true
		clientPool.Unlock()
		log.Printf("[WebSocket] 新客户端连接，当前在线数：%d", len(clientPool.conns))

		// 3. 启动读消息 Goroutine（分离读逻辑，不阻塞）
		go readMsg(conn)

		// 4. 启动心跳检测（保持连接）
		go startHeartbeat(conn)

		// 标记请求已处理，阻止 Gin 后续中间件干扰
		c.Abort()
	}
}

// Broadcast：全局广播函数（任意地方可调用，向所有在线客户端推送）
// msgType：1=文本，2=二进制
func Broadcast(msgType int, data []byte) {
	clientPool.RLock()
	defer clientPool.RUnlock()

	for conn := range clientPool.conns {
		// 并发写保护：同一连接避免同时写
		if err := conn.WriteMessage(msgType, data); err != nil {
			log.Printf("[WebSocket] 广播失败（客户端断开）：%v", err)
			// 清理失效连接
			clientPool.RUnlock()
			removeConn(conn)
			clientPool.RLock()
		}
	}
	log.Printf("[WebSocket] 广播完成：%s（在线数：%d）", string(data), len(clientPool.conns))
}

// 辅助函数：判断是否为 WebSocket 握手请求
func isWebSocketHandshake(req *http.Request) bool {
	return req.Header.Get("Upgrade") == "websocket" && req.Header.Get("Connection") == "Upgrade"
}

// 辅助函数：读取客户端消息（异常时移除连接）
func readMsg(conn *websocket.Conn) {
	defer func() {
		removeConn(conn)
		log.Printf("[WebSocket] 客户端断开，当前在线数：%d", len(clientPool.conns))
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WebSocket] 读取消息失败：%v", err)
			return
		}
		log.Printf("[WebSocket] 收到客户端消息：%s", string(data))

		// 可选：客户端消息触发广播（回声+广播）
		Broadcast(websocket.TextMessage, []byte("客户端广播："+string(data)))
	}
}

// 辅助函数：心跳检测（避免连接被网关断开）
func startHeartbeat(conn *websocket.Conn) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for range ticker.C {
		heartbeatMsg := "心跳检测：" + time.Now().Format("15:04:05")
		if err := conn.WriteMessage(websocket.TextMessage, []byte(heartbeatMsg)); err != nil {
			log.Printf("[WebSocket] 心跳推送失败：%v", err)
			removeConn(conn)
			return
		}
	}
}

// 辅助函数：从连接池移除并关闭连接（线程安全）
func removeConn(conn *websocket.Conn) {
	clientPool.Lock()
	defer clientPool.Unlock()
	delete(clientPool.conns, conn)
	_ = conn.Close() // 忽略关闭错误
}
