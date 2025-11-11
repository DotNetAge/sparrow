package messaging

import (
	"context"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/nats-io/nats.go"
)

type StreamHub struct {
	usecase.GracefulClose
	usecase.Startable
	Subscribers Subscribers
	localName   string // 本服务名，用作消费者的唯一名
	serviceName string
	logger      *logger.Logger
}

// NewStreamBus 创建一个新的事件流总线
func NewStreamBus(conn *nats.Conn,
	localName string,
	serviceName string,
	logger *logger.Logger) *StreamHub {
	subscribers := NewJetStreamBus(conn, localName, serviceName, logger)
	return &StreamHub{
		localName:   localName,
		serviceName: serviceName,
		Subscribers: subscribers.(Subscribers),
		logger:      logger,
	}
}

func (b *StreamHub) AddSub(aggType, eventType string, handler DomainEventHandler[*entity.BaseEvent]) {
	b.Subscribers.AddHandler(aggType, eventType, handler)
}

// Close 实现GracefulClose接口，支持优雅关闭
func (b *StreamHub) Close(ctx context.Context) error {
	return b.Subscribers.(usecase.GracefulClose).Close(ctx)
}

// Start 实现Startable接口，支持启动
func (b *StreamHub) Start(ctx context.Context) error {
	return b.Subscribers.(usecase.Startable).Start(ctx)
}
