package entity

import (
	"time"
)

// Event 通用事件接口，是所有事件的基础接口
type Event interface {
	GetEventID() string      // 事件唯一标识符
	GetEventType() string    // 事件类型
	GetCreatedAt() time.Time // 事件创建时间
}
