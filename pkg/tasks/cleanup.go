package tasks

import (
	"time"
)

// CleanupPolicy 清理策略配置
type CleanupPolicy struct {
	// CompletedTaskTTL 已完成任务保留时间
	CompletedTaskTTL time.Duration
	// FailedTaskTTL 失败任务保留时间  
	FailedTaskTTL time.Duration
	// CancelledTaskTTL 已取消任务保留时间
	CancelledTaskTTL time.Duration
	// MaxCompletedTasks 最大已完成任务数量（超过后按LRU清理）
	MaxCompletedTasks int
	// MaxFailedTasks 最大失败任务数量
	MaxFailedTasks int
	// CleanupInterval 清理检查间隔
	CleanupInterval time.Duration
	// EnableAutoCleanup 是否启用自动清理
	EnableAutoCleanup bool
}

// DefaultCleanupPolicy 返回默认清理策略
func DefaultCleanupPolicy() *CleanupPolicy {
	return &CleanupPolicy{
		CompletedTaskTTL:   30 * time.Minute,  // 已完成任务保留30分钟
		FailedTaskTTL:      60 * time.Minute,  // 失败任务保留60分钟
		CancelledTaskTTL:   15 * time.Minute,  // 已取消任务保留15分钟
		MaxCompletedTasks:  1000,             // 最多保留1000个已完成任务
		MaxFailedTasks:     500,              // 最多保留500个失败任务
		CleanupInterval:    5 * time.Minute,   // 每5分钟检查一次
		EnableAutoCleanup:  true,             // 默认启用自动清理
	}
}

// ShouldCleanup 判断任务是否应该被清理
func (p *CleanupPolicy) ShouldCleanup(status TaskStatus, createdAt, updatedAt time.Time, currentTaskCount int) bool {
	if !p.EnableAutoCleanup {
		return false
	}

	now := time.Now()
	
	switch status {
	case TaskStatusCompleted:
		// 检查TTL
		if now.Sub(updatedAt) > p.CompletedTaskTTL {
			return true
		}
		// 检查最大数量限制
		if p.MaxCompletedTasks > 0 && currentTaskCount > p.MaxCompletedTasks {
			return true
		}
		
	case TaskStatusFailed:
		// 检查TTL
		if now.Sub(updatedAt) > p.FailedTaskTTL {
			return true
		}
		// 检查最大数量限制
		if p.MaxFailedTasks > 0 && currentTaskCount > p.MaxFailedTasks {
			return true
		}
		
	case TaskStatusCancelled:
		// 已取消任务只检查TTL
		if now.Sub(updatedAt) > p.CancelledTaskTTL {
			return true
		}
	}
	
	return false
}