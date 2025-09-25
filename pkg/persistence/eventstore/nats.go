package eventstore

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/usecase"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NatsEventStore 基于NATS JetStream的事件存储实现
type NatsEventStore struct {
	js          jetstream.JetStream
	nc          *nats.Conn
	logger      *logger.Logger
	streamName  string
	storeStream string
	bucketName  string
	durableName string
}

var _ usecase.EventStore = (*NatsEventStore)(nil)

// NewNatsEventStore 创建NATS事件存储实例
func NewNatsEventStore(cfg *config.NATsConfig, logger *logger.Logger) (usecase.EventStore, error) {
	// 连接到NATS服务器
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		if logger != nil {
			logger.Error("连接到NATS服务器失败", "error", err)
		}
		return nil, fmt.Errorf("连接到NATS服务器失败: %w", err)
	}

	// 获取JetStream上下文
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		if logger != nil {
			logger.Error("获取JetStream上下文失败", "error", err)
		}
		return nil, fmt.Errorf("获取JetStream上下文失败: %w", err)
	}

	// 创建事件存储流
	if err := createEventStoreStream(context.Background(), js, cfg.StoreStream); err != nil {
		nc.Close()
		if logger != nil {
			logger.Error("创建事件存储流失败", "error", err)
		}
		return nil, fmt.Errorf("创建事件存储流失败: %w", err)
	}

	return &NatsEventStore{
		js:          js,
		nc:          nc,
		logger:      logger,
		streamName:  cfg.StreamName,
		storeStream: cfg.StoreStream,
		bucketName:  cfg.BucketName,
		durableName: cfg.DurableName,
	}, nil
}

// createEventStoreStream 创建事件存储所需的JetStream流
func createEventStoreStream(ctx context.Context, js jetstream.JetStream, streamName string) error {
	// 检查流是否已存在
	_, err := js.Stream(ctx, streamName)
	if err == nil {
		// 流已存在，返回成功
		fmt.Printf("流 %s 已存在\n", streamName)
		return nil
	}

	// 尝试创建流，配置更加详细
	streamConfig := jetstream.StreamConfig{
		Name:      streamName,
		Subjects:  []string{streamName + ".>"}, // 使用>通配符匹配所有以streamName开头的主题
		Storage:   jetstream.FileStorage,
		Replicas:  1,
		MaxBytes:  1024 * 1024 * 1024, // 1GB
		MaxMsgs:   -1,                 // 无限制
		Retention: jetstream.LimitsPolicy,
	}

	_, err = js.CreateStream(ctx, streamConfig)
	if err != nil {
		return fmt.Errorf("创建流失败: %w", err)
	}

	fmt.Printf("流 %s 创建成功\n", streamName)
	// 等待一小段时间，确保流已完全初始化
	return nil
}

// SaveEvents 保存事件
func (s *NatsEventStore) SaveEvents(ctx context.Context, aggregateID string, events []entity.DomainEvent, expectedVersion int) error {
	if len(events) == 0 {
		return nil
	}

	// 首先检查流是否存在并且可用
	_, err := s.js.Stream(ctx, s.storeStream)
	if err != nil {
		fmt.Printf("检查流 %s 失败: %v\n", s.storeStream, err)
		// 尝试重新创建流
		if err := createEventStoreStream(ctx, s.js, s.storeStream); err != nil {
			fmt.Printf("重新创建流失败: %v\n", err)
			return fmt.Errorf("流不可用: %w", err)
		}
	} else {
		fmt.Printf("流 %s 存在且可用\n", s.storeStream)
	}

	// 检查当前版本
	currentVersion, err := s.GetAggregateVersion(ctx, aggregateID)
	if err != nil {
		return fmt.Errorf("获取当前版本失败: %w", err)
	}

	if expectedVersion != -1 && currentVersion != expectedVersion {
		return &errs.EventStoreError{
			Type:      "concurrency_conflict",
			Message:   fmt.Sprintf("预期版本 %d, 实际版本 %d", expectedVersion, currentVersion),
			Aggregate: aggregateID,
		}
	}

	// 保存事件
	for i, event := range events {
		version := currentVersion + i + 1
		eventBytes, err := EncodeEvent(event)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("编码事件失败", "aggregate_id", aggregateID, "event_type", event.GetEventType(), "error", err)
			}
			return fmt.Errorf("编码事件失败: %w", err)
		}

		subject := fmt.Sprintf("%s.%s.%d", s.storeStream, aggregateID, version)
		if s.logger != nil {
			s.logger.Debug("尝试发布事件到主题", "subject", subject, "aggregate_id", aggregateID, "event_type", event.GetEventType(), "version", version)
		}
		
		// 使用标准Publish方法发布事件
		_, err = s.js.Publish(ctx, subject, eventBytes)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("发布事件失败", "aggregate_id", aggregateID, "event_type", event.GetEventType(), "version", version, "error", err)
			}
			return fmt.Errorf("发布事件失败: %w", err)
		}
		
		if s.logger != nil {
			s.logger.Debug("成功发布事件到主题", "subject", subject)
		}
	}

	return nil
}

// GetEvents 获取聚合的所有事件
func (s *NatsEventStore) GetEvents(ctx context.Context, aggregateID string) ([]entity.DomainEvent, error) {
	return s.getEventsBySubject(ctx, fmt.Sprintf("%s.%s.*", s.storeStream, aggregateID))
}

// GetEventsFromVersion 从指定版本开始获取事件
func (s *NatsEventStore) GetEventsFromVersion(ctx context.Context, aggregateID string, fromVersion int) ([]entity.DomainEvent, error) {
	return s.getEventsBySubjectWithFilter(ctx, fmt.Sprintf("%s.%s.*", s.storeStream, aggregateID), func(msg jetstream.Msg) bool {
		// 解析消息中的事件元数据
		var meta EventMeta
		if err := json.Unmarshal(msg.Data(), &meta); err != nil {
			return false
		}
		// 只返回版本号大于或等于fromVersion的事件
		return meta.Version >= fromVersion
	})
}

// GetEventsByType 按事件类型获取事件
func (s *NatsEventStore) GetEventsByType(ctx context.Context, eventType string) ([]entity.DomainEvent, error) {
	// 在NATS中按类型过滤事件效率不高，我们需要获取所有事件然后在客户端过滤
	return s.getEventsBySubjectWithFilter(ctx, fmt.Sprintf("%s.*.*", s.storeStream), func(msg jetstream.Msg) bool {
		var meta EventMeta
		if err := json.Unmarshal(msg.Data(), &meta); err != nil {
			return false
		}
		return meta.EventType == eventType
	})
}

// GetEventsByTimeRange 按时间范围获取事件
func (s *NatsEventStore) GetEventsByTimeRange(ctx context.Context, aggregateID string, fromTime, toTime time.Time) ([]entity.DomainEvent, error) {
	return s.getEventsBySubjectWithFilter(ctx, fmt.Sprintf("%s.%s.*", s.storeStream, aggregateID), func(msg jetstream.Msg) bool {
		var meta EventMeta
		if err := json.Unmarshal(msg.Data(), &meta); err != nil {
			return false
		}
		return meta.CreatedAt.After(fromTime) && meta.CreatedAt.Before(toTime)
	})
}

// GetEventsWithPagination 分页获取事件
func (s *NatsEventStore) GetEventsWithPagination(ctx context.Context, aggregateID string, limit, offset int) ([]entity.DomainEvent, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	allEvents, err := s.GetEvents(ctx, aggregateID)
	if err != nil {
		return nil, err
	}

	// 实现内存分页
	start := offset
	end := offset + limit
	if start >= len(allEvents) {
		return []entity.DomainEvent{}, nil
	}
	if end > len(allEvents) {
		end = len(allEvents)
	}

	return allEvents[start:end], nil
}

// GetAggregateVersion 获取聚合版本
func (s *NatsEventStore) GetAggregateVersion(ctx context.Context, aggregateID string) (int, error) {
	events, err := s.GetEvents(ctx, aggregateID)
	if err != nil {
		return 0, err
	}

	maxVersion := 0
	for _, event := range events {
		if event.GetVersion() > maxVersion {
			maxVersion = event.GetVersion()
		}
	}

	return maxVersion, nil
}

// Load 从事件流加载聚合根状态
func (s *NatsEventStore) Load(ctx context.Context, aggregateID string, aggregate entity.AggregateRoot) error {
	// 1. 尝试从快照恢复
	snapshot, version, err := s.GetLatestSnapshot(ctx, aggregateID)
	var events []entity.DomainEvent

	// 2. 获取快照之后的事件
	if err == nil && snapshot != nil {
		// 将快照数据应用到聚合根
		if err := aggregate.LoadFromSnapshot(snapshot); err != nil {
			if s.logger != nil {
				s.logger.Error("从快照加载失败", "aggregate_id", aggregateID, "error", err)
			}
			return fmt.Errorf("从快照加载失败: %w", err)
		}
		// 获取快照版本之后的事件
		if version > 0 {
			events, err = s.GetEventsFromVersion(ctx, aggregateID, version+1)
			if err != nil {
				if s.logger != nil {
					s.logger.Error("从指定版本获取事件失败", "aggregate_id", aggregateID, "from_version", version+1, "error", err)
				}
				return fmt.Errorf("从指定版本获取事件失败: %w", err)
			}
		}
	} else {
		// 没有快照或快照加载失败，从所有事件恢复
		events, err = s.GetEvents(ctx, aggregateID)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("获取事件失败", "aggregate_id", aggregateID, "error", err)
			}
			return fmt.Errorf("获取事件失败: %w", err)
		}
	}

	// 3. 应用事件到聚合根
	if len(events) > 0 {
		if err := aggregate.LoadFromEvents(events); err != nil {
			if s.logger != nil {
				s.logger.Error("从事件加载失败", "aggregate_id", aggregateID, "events_count", len(events), "error", err)
			}
			return fmt.Errorf("从事件加载失败: %w", err)
		}
	}

	return nil
}

// SaveSnapshot 保存快照
func (s *NatsEventStore) SaveSnapshot(ctx context.Context, aggregateID string, snapshot interface{}, version int) error {
	snapshotData, err := json.Marshal(snapshot)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("编码快照失败", "aggregate_id", aggregateID, "version", version, "error", err)
		}
		return fmt.Errorf("编码快照失败: %w", err)
	}

	subject := fmt.Sprintf("%s.snapshot.%s", s.storeStream, aggregateID)
	if _, err := s.js.Publish(ctx, subject, snapshotData); err != nil {
		if s.logger != nil {
			s.logger.Error("保存快照失败", "aggregate_id", aggregateID, "version", version, "error", err)
		}
		return fmt.Errorf("保存快照失败: %w", err)
	}

	return nil
}

// GetLatestSnapshot 获取最新快照
func (s *NatsEventStore) GetLatestSnapshot(ctx context.Context, aggregateID string) (interface{}, int, error) {
	subject := fmt.Sprintf("%s.snapshot.%s", s.storeStream, aggregateID)

	// 在NATS中，我们可以使用Consumer来获取最新的快照消息
	consumerName := fmt.Sprintf("snapshot_consumer_%s", aggregateID)

	// 创建消费者
	consumer, err := s.js.CreateOrUpdateConsumer(ctx, s.storeStream, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: subject,
		DeliverPolicy: jetstream.DeliverLastPolicy,
	})
	if err != nil {
		if s.logger != nil {
			s.logger.Error("创建快照消费者失败", "aggregate_id", aggregateID, "error", err)
		}
		return nil, 0, fmt.Errorf("创建快照消费者失败: %w", err)
	}

	// 获取最新消息
	msgs, err := consumer.Fetch(1)
	if err != nil {
		// 没有快照消息也是正常的
		return nil, 0, nil
	}

	var snapshotData []byte
	for msg := range msgs.Messages() {
		defer msg.Ack()
		snapshotData = msg.Data()
		break
	}

	if len(snapshotData) == 0 {
		return nil, 0, nil
	}

	// 从消息中提取版本信息 (这里我们假设版本信息在快照数据中)
	var snapshot interface{}
	if err := json.Unmarshal(snapshotData, &snapshot); err != nil {
		if s.logger != nil {
			s.logger.Error("解码快照失败", "aggregate_id", aggregateID, "error", err)
		}
		return nil, 0, fmt.Errorf("解码快照失败: %w", err)
	}

	// 假设版本是存储在聚合根中的，这里我们无法直接获取
	// 在实际使用中，可能需要修改快照结构以包含版本信息
	version, err := s.GetAggregateVersion(ctx, aggregateID)
	if err != nil {
		version = 0
	}

	return snapshot, version, nil
}

// SaveEventsBatch 批量保存事件
func (s *NatsEventStore) SaveEventsBatch(ctx context.Context, events map[string][]entity.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}

	for aggregateID, eventList := range events {
		if err := s.SaveEvents(ctx, aggregateID, eventList, -1); err != nil {
			if s.logger != nil {
				s.logger.Error("批量保存事件失败", "aggregate_id", aggregateID, "error", err)
			}
			return fmt.Errorf("批量保存事件失败: %w", err)
		}
	}

	return nil
}

// Close 关闭连接
func (s *NatsEventStore) Close() error {
	if s.nc != nil {
		s.nc.Close()
	}
	return nil
}

// getEventsBySubject 辅助方法：通过主题获取事件
func (s *NatsEventStore) getEventsBySubject(ctx context.Context, subject string) ([]entity.DomainEvent, error) {
	return s.getEventsBySubjectWithFilter(ctx, subject, func(_ jetstream.Msg) bool {
		return true
	})
}

// getEventsBySubjectWithFilter 辅助方法：通过主题获取事件并应用过滤器
func (s *NatsEventStore) getEventsBySubjectWithFilter(ctx context.Context, subject string, filter func(jetstream.Msg) bool) ([]entity.DomainEvent, error) {
	// 使用更安全的方式生成消费者名称，避免使用特殊字符
	// 这里我们简单地使用前缀加上时间戳和随机数来确保唯一性
	consumerName := fmt.Sprintf("consumer_%d_%d", time.Now().UnixNano(), rand.Int63())
	consumer, err := s.js.CreateOrUpdateConsumer(ctx, s.storeStream, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: subject,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		if s.logger != nil {
			s.logger.Error("创建消费者失败", "subject", subject, "error", err)
		}
		return nil, fmt.Errorf("创建消费者失败: %w", err)
	}

	// 获取所有消息
	var events []entity.DomainEvent

	// 设置Fetch选项，添加超时
	fetchOptions := []jetstream.FetchOpt{
		jetstream.FetchMaxWait(5 * time.Second),
	}

	msgs, err := consumer.Fetch(1000, fetchOptions...) // 一次获取最多1000条消息
	if err != nil {
		// 如果没有消息或超时，返回空列表
		if err == nats.ErrTimeout || strings.Contains(err.Error(), "no messages") {
			return []entity.DomainEvent{}, nil
		}
		if s.logger != nil {
			s.logger.Error("获取消息失败", "error", err)
		}
		return nil, fmt.Errorf("获取消息失败: %w", err)
	}

	for msg := range msgs.Messages() {
		defer msg.Ack()

		// 应用过滤器
		if !filter(msg) {
			continue
		}

		// 解码事件
		event, err := DecodeEvent(msg.Data())
		if err != nil {
			if s.logger != nil {
				s.logger.Error("解码事件失败", "error", err)
			}
			continue
		}
		events = append(events, event)
	}

	return events, nil
}
