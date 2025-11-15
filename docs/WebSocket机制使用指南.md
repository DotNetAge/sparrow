# WebSocket机制使用指南

## 🚀 功能特性

### 核心功能
- **单点发送**：`SendTextToClient(clientID, text)`, `SendJSONToClient(clientID, data)`
- **用户广播**：`SendTextToUser(userID, text)`, `SendJSONToUser(userID, data)`
- **全局广播**：`BroadcastText(text)`, `BroadcastJSON(data)`
- **在线统计**：`GetOnlineCount()`
- **连接管理**：自动连接池、心跳检测、断线重连

### 消息类型
- `MessageText`: 文本消息
- `MessageJSON`: JSON消息
- `MessageBinary`: 二进制消息（文件、图片等）
- `MessageHeartbeat`: 心跳消息（内部使用）

### 便捷接口 vs 高级接口

#### 便捷接口（推荐日常使用）
```go
// 文本消息
ws.BroadcastText("Hello World")
ws.SendTextToClient("client_123", "Private message")
ws.SendTextToUser("user_456", "User message")

// JSON消息
ws.BroadcastJSON(map[string]interface{}{"type": "notice", "content": "System update"})
ws.SendJSONToClient("client_123", userData)
ws.SendJSONToUser("user_456", notification)

// 二进制数据
ws.SendBinaryToClient("client_123", fileData)
```

#### 高级接口（向后兼容）
```go
// 使用原始WebSocket消息类型
ws.Broadcast(websocket.TextMessage, []byte("Hello"))
ws.SendToClient("client_123", websocket.TextMessage, []byte("Message"))
ws.SendToUser("user_456", websocket.BinaryMessage, binaryData)

// 使用新的MessageType枚举
ws.BroadcastMessage(ws.MessageText, []byte("Hello"))
ws.SendToClientMessage("client_123", ws.MessageJSON, jsonData)
ws.SendToUserMessage("user_456", ws.MessageBinary, binaryData)
```

## 📋 快速开始

### 1. 服务端集成

```go
package main

import (
    "log"
    "time"
    "github.com/gin-gonic/gin"
    "your-project/pkg/ws"
)

func main() {
    // 1. 初始化 Gin 引擎
    r := gin.Default()

    // 2. 载入 WebSocket 中间件
    r.Use(ws.Middleware(
        // 可选：自定义跨域规则
        ws.WithCheckOrigin(func(r *http.Request) bool {
            origin := r.Header.Get("Origin")
            return origin == "http://localhost:3000" || origin == ""
        }),
        // 可选：自定义心跳间隔
        ws.WithHeartbeatInterval(15*time.Second),
    ))

    // 3. 注册 WebSocket 路由
    r.GET("/ws", func(c *gin.Context) {
        // 中间件已处理所有WebSocket逻辑
        log.Println("WebSocket连接建立")
    })

    // 4. 添加消息发送API
    r.POST("/api/send-text", func(c *gin.Context) {
        var req struct {
            Message string `json:"message" binding:"required"`
        }
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(400, gin.H{"error": "Invalid request"})
            return
        }

        // 使用便捷的文本广播接口
        if err := ws.BroadcastText(req.Message); err != nil {
            c.JSON(500, gin.H{"error": "Broadcast failed"})
            return
        }
        
        c.JSON(200, gin.H{"status": "success"})
    })

    // 5. 启动服务
    log.Println("服务启动: http://localhost:8080")
    log.Println("WebSocket地址: ws://localhost:8080/ws")
    r.Run(":8080")
}
```

### 2. 客户端连接

#### JavaScript原生客户端
```javascript
// 连接WebSocket
const ws = new WebSocket('ws://localhost:8080/ws?user_id=123&client_id=client_123');

ws.onopen = function(event) {
    console.log('WebSocket连接已建立');
};

ws.onmessage = function(event) {
    console.log('收到消息:', event.data);
    
    // 解析JSON消息
    try {
        const data = JSON.parse(event.data);
        console.log('JSON数据:', data);
    } catch (e) {
        console.log('文本消息:', event.data);
    }
};

ws.onclose = function(event) {
    console.log('WebSocket连接已关闭');
};

ws.onerror = function(error) {
    console.error('WebSocket错误:', error);
};

// 发送消息
ws.send('Hello Server!');
```

## 🛠️ 高级用法

### 1. 消息类型处理

```go
// 发送不同类型的消息
func sendMessageExample() {
    // 文本消息
    ws.BroadcastText("简单文本消息")
    
    // JSON消息
    data := map[string]interface{}{
        "type": "notification",
        "user": "张三",
        "content": "您有新消息",
        "timestamp": time.Now().Unix(),
    }
    ws.BroadcastJSON(data)
    
    // 二进制数据（如图片）
    imageData, _ := os.ReadFile("image.jpg")
    ws.SendBinaryToClient("client_123", imageData)
}
```

### 2. 定时任务实现

```go
// 定时广播系统消息
func startSystemBroadcast() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        stats := map[string]interface{}{
            "type": "system_stats",
            "online_count": ws.GetOnlineCount(),
            "server_time": time.Now().Format("2006-01-02 15:04:05"),
            "timestamp": time.Now().Unix(),
        }
        
        // 使用JSON广播发送统计数据
        if err := ws.BroadcastJSON(stats); err != nil {
            log.Printf("统计广播失败: %v", err)
        }
    }
}
```

### 3. 用户消息路由

```go
// 根据用户类型发送不同消息
func sendUserNotification(userID string, userType string) {
    switch userType {
    case "vip":
        // VIP用户发送富文本消息
        vipMessage := map[string]interface{}{
            "type": "vip_notification",
            "content": "VIP专享通知",
            "priority": "high",
            "badge": true,
        }
        ws.SendJSONToUser(userID, vipMessage)
        
    case "normal":
        // 普通用户发送简单文本
        ws.SendTextToUser(userID, "系统通知：您有新消息")
        
    default:
        // 默认文本消息
        ws.BroadcastText("通用系统通知")
    }
}
```

### 4. 文件传输示例

```go
// 文件上传和广播
func handleFileUpload(c *gin.Context) {
    file, header, err := c.Request.FormFile("file")
    if err != nil {
        c.JSON(400, gin.H{"error": "文件上传失败"})
        return
    }
    defer file.Close()

    // 读取文件内容
    fileData := make([]byte, header.Size)
    if _, err := file.Read(fileData); err != nil {
        c.JSON(500, gin.H{"error": "文件读取失败"})
        return
    }

    // 广播文件信息
    fileInfo := map[string]interface{}{
        "type": "file_broadcast",
        "filename": header.Filename,
        "size": header.Size,
        "content_type": header.Header.Get("Content-Type"),
    }
    
    // 先发送文件信息
    ws.BroadcastJSON(fileInfo)
    
    // 再发送文件内容（二进制）
    ws.BroadcastMessage(ws.MessageBinary, fileData)
    
    c.JSON(200, gin.H{"status": "success", "message": "文件已广播"})
}
```

## 📚 API参考

### 便捷接口

#### 全局广播
```go
// 文本广播
func BroadcastText(text string) error

// JSON广播
func BroadcastJSON(data interface{}) error
```

#### 单点发送
```go
// 发送文本给指定客户端
func SendTextToClient(clientID string, text string) error

// 发送JSON给指定客户端
func SendJSONToClient(clientID string, data interface{}) error

// 发送二进制数据给指定客户端
func SendBinaryToClient(clientID string, data []byte) error
```

#### 用户发送
```go
// 发送文本给指定用户的所有客户端
func SendTextToUser(userID string, text string) error

// 发送JSON给指定用户的所有客户端
func SendJSONToUser(userID string, data interface{}) error

// 发送二进制数据给指定用户的所有客户端
func SendBinaryToUser(userID string, data []byte) error
```

### 高级接口

#### 使用MessageType枚举
```go
// 使用枚举类型广播
func BroadcastMessage(msgType MessageType, data []byte) error

// 使用枚举类型单点发送
func SendToClientMessage(clientID string, msgType MessageType, data []byte) error

// 使用枚举类型用户发送
func SendToUserMessage(userID string, msgType MessageType, data []byte) error
```

#### 向后兼容接口
```go
// 原始WebSocket消息类型（保持兼容）
func Broadcast(messageType int, data []byte) error
func SendToClient(clientID string, msgType int, data []byte) bool
func SendToUser(userID string, messageType int, data []byte) error
```

### 工具函数
```go
// 获取在线客户端数量
func GetOnlineCount() int

// 获取所有客户端信息
func GetAllClients() []*Client
```

## 🎯 最佳实践

### 1. 消息类型选择
- **文本消息**：简单通知、聊天内容、系统消息
- **JSON消息**：结构化数据、API响应、复杂通知
- **二进制消息**：文件、图片、音视频

### 2. 错误处理
```go
// 推荐的错误处理方式
func safeBroadcast() {
    if err := ws.BroadcastText("Hello"); err != nil {
        log.Printf("广播失败: %v", err)
        // 可以选择重试或记录失败
    }
}
```

### 3. 性能优化
- 大文件分块传输
- JSON数据压缩
- 消息队列缓冲
- 连接池复用

### 4. 安全考虑
- 客户端身份验证
- 消息内容过滤
- 频率限制
- 跨域控制

## 🔧 故障排除

### 常见问题

#### 1. 连接失败
```go
// 检查跨域配置
r.Use(ws.Middleware(
    ws.WithCheckOrigin(func(r *http.Request) bool {
        // 允许的域名
        return r.Header.Get("Origin") == "https://yourdomain.com"
    }),
))
```

#### 2. 消息发送失败
```go
// 检查客户端是否在线
if ws.GetOnlineCount() == 0 {
    log.Println("没有在线客户端")
    return
}
```

#### 3. JSON序列化错误
```go
// 确保数据可序列化
data := map[string]interface{}{
    "name": "张三",
    "age":  25,
    "active": true,
}
// 避免包含函数、channel等不可序列化的类型
```

## 📱 Vue + TypeScript 客户端

详细的Vue + TypeScript客户端实现请参考文档中的完整示例代码，包括：
- WebSocket连接管理
- 自动重连机制
- 消息类型处理
- 响应式状态管理
- UI组件集成

---

## 📖 更新日志

### v2.0.0 - 便捷接口重构
- ✅ 新增MessageType枚举类型
- ✅ 新增便捷发送接口（SendText、SendJSON、SendBinary等）
- ✅ 保持向后兼容性
- ✅ 优化错误处理
- ✅ 更新使用文档和示例
- ✅ 移除冗余的BroadcastSystem函数

### v1.0.0 - 基础功能
- ✅ WebSocket连接管理
- ✅ 消息广播和单点发送
- ✅ 用户消息路由
- ✅ 心跳检测
- ✅ 连接池管理