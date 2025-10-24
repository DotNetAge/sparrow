package usecase

import "context"

// GracefulClose 优雅关闭接口
// 应用于非阻塞式进程，并能在收到关闭信号后，完成当前任务后关闭，用于清理资源等
type GracefulClose interface {
	Close(ctx context.Context) error
}

type Startable interface {
	Start(ctx context.Context) error
}
