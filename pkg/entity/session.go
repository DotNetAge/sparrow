package entity

import (
	"time"
)

// Session 会话实体
// 位于整洁架构的核心层 - 实体层
// 包含会话的核心业务数据和规则

type Session struct {
	BaseEntity                        // 嵌入基础实体，继承标准字段和方法
	Data       map[string]interface{} `json:"data"`
	Expiry     time.Time              `json:"expiry"`
}

// 确保Session实现了Entity接口
var _ Entity = (*Session)(nil)

// NewSession 创建新会话实体
func NewSession(id string, ttl time.Duration) *Session {
	return &Session{
		BaseEntity: *NewBaseEntity(id),
		Data:       make(map[string]interface{}),
		Expiry:     time.Now().Add(ttl),
	}
}

// IsExpired 检查会话是否过期
// 核心业务规则：当前时间超过过期时间则视为过期
func (s *Session) IsExpired() bool {
	return time.Now().After(s.Expiry)
}

// UpdateExpiry 更新会话过期时间
func (s *Session) UpdateExpiry(ttl time.Duration) {
	s.Expiry = time.Now().Add(ttl)
}

// SetData 设置会话数据
func (s *Session) SetData(key string, value interface{}) {
	if s.Data == nil {
		s.Data = make(map[string]interface{})
	}
	s.Data[key] = value
}

// GetData 获取会话数据
func (s *Session) GetData(key string) (interface{}, bool) {
	if s.Data == nil {
		return nil, false
	}
	val, exists := s.Data[key]
	return val, exists
}

// DeleteData 删除会话数据
func (s *Session) DeleteData(key string) {
	if s.Data != nil {
		delete(s.Data, key)
	}
}
