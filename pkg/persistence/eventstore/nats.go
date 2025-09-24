package eventstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/utils"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NATSEventStore Nats事件存储实现（永久存储）
type NATSEventStore struct {
	conn            *nats.Conn
	js              jetstream.JetStream
	streamName      string
	eventSubject    string
	snapshotSubject string
	bucketName      string
	bucket          jetstream.KeyValue
	logger          *logger.Logger
}

// NewNATSEventStore 创建Nats事件存储实例
func NewNATSEventStore(cfg *config.NATsConfig, log *logger.Logger) (*NATSEventStore, error) {
	conn, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := jetstream.New(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create jetstream: %w", err)
	}

	store := &NATSEventStore{
		conn:            conn,
		js:              js,
		streamName:      cfg.StoreStream,
		eventSubject:    fmt.Sprintf("%s.events", cfg.StoreStream),
		snapshotSubject: fmt.Sprintf("%s.snapshots", cfg.StoreStream),
		bucketName:      cfg.BucketName,
		logger:          log,
	}

	if err := store.createResources(); err != nil {
		return nil, fmt.Errorf("failed to create resources: %w", err)
	}

	bucket, err := js.KeyValue(context.Background(), cfg.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get key-value bucket: %w", err)
	}
	store.bucket = bucket

	return store, nil
}

// createResources 创建必要的Nats资源
func (s *NATSEventStore) createResources() error {
	ctx := context.Background()

	// 创建事件流
	_, err := s.js.CreateStream(ctx, jetstream.StreamConfig{
		Name:        s.streamName,
		Description: "Event store stream",
		Subjects:    []string{fmt.Sprintf("%s.*", s.streamName)},
		Retention:   jetstream.LimitsPolicy,
		Storage:     jetstream.FileStorage,
		Replicas:    1,
	})
	if err != nil && !strings.Contains(err.Error(), "stream name already in use") {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	// 创建Key-Value桶用于存储快照和版本信息
	_, err = s.js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:      s.bucketName,
		Description: "Event store snapshots and metadata",
		Storage:     jetstream.FileStorage,
		Replicas:    1,
	})
	if err != nil && !strings.Contains(err.Error(), "bucket name already in use") {
		return fmt.Errorf("failed to create key-value bucket: %w", err)
	}

	return nil
}

// SaveEvents 保存事件
func (s *NATSEventStore) SaveEvents(ctx context.Context, aggregateID string, events []entity.DomainEvent, expectedVersion int) error {
	if len(events) == 0 {
		return nil
	}

	// 获取当前版本
	currentVersion, err := s.GetAggregateVersion(ctx, aggregateID)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if expectedVersion != -1 && currentVersion != expectedVersion {
		return &errs.EventStoreError{
			Type:      "concurrency_conflict",
			Message:   fmt.Sprintf("expected version %d, got %d", expectedVersion, currentVersion),
			Aggregate: aggregateID,
		}
	}

	// 发布事件到JetStream
	for i, event := range events {
		eventData, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		version := currentVersion + i + 1

		// 创建事件消息
		msg := map[string]interface{}{
			"aggregate_id": aggregateID,
			"event_type":   event.GetEventType(),
			"event_data":   string(eventData),
			"version":      version,
			"timestamp":    time.Now().Unix(),
		}

		msgData, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal event message: %w", err)
		}

		// 发布到JetStream
		subject := fmt.Sprintf("%s.%s", s.eventSubject, aggregateID)
		_, err = s.js.Publish(ctx, subject, msgData)
		if err != nil {
			return fmt.Errorf("failed to publish event: %w", err)
		}
	}

	// 更新版本信息
	versionKey := s.versionKey(aggregateID)
	newVersion := currentVersion + len(events)
	_, err = s.bucket.Put(ctx, versionKey, []byte(strconv.Itoa(newVersion)))
	if err != nil {
		return fmt.Errorf("failed to update version: %w", err)
	}

	return nil
}

// GetEvents 获取聚合的所有事件
func (s *NATSEventStore) GetEvents(ctx context.Context, aggregateID string) ([]entity.DomainEvent, error) {
	return s.getEventsFromVersion(ctx, aggregateID, 1)
}

// GetEventsFromVersion 从指定版本开始获取事件
func (s *NATSEventStore) GetEventsFromVersion(ctx context.Context, aggregateID string, fromVersion int) ([]entity.DomainEvent, error) {
	return s.getEventsFromVersion(ctx, aggregateID, fromVersion)
}

// getEventsFromVersion 内部获取事件方法
func (s *NATSEventStore) getEventsFromVersion(ctx context.Context, aggregateID string, fromVersion int) ([]entity.DomainEvent, error) {
	// 使用JetStream的消费者获取事件
	subject := fmt.Sprintf("%s.%s", s.eventSubject, aggregateID)

	// 创建消费者（如果存在则获取）
	consumer, err := s.js.CreateOrUpdateConsumer(ctx, s.streamName, jetstream.ConsumerConfig{
		Durable:       fmt.Sprintf("consumer-%s", aggregateID),
		FilterSubject: subject,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	// 获取消息
	messages, err := consumer.Fetch(1000) // 限制单次获取数量
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	var events []entity.DomainEvent
	currentVersion := 1

	for msg := range messages.Messages() {
		var eventMsg map[string]interface{}
		if err := json.Unmarshal(msg.Data(), &eventMsg); err != nil {
			if err := msg.Nak(); err != nil {
				s.logger.Error("Failed to Nak message", "error", err)
			}
			continue
		}

		if currentVersion >= fromVersion {
			// 解析事件数据
			eventData := eventMsg["event_data"].(string)
			eventType := eventMsg["event_type"].(string)
			version := int(eventMsg["version"].(float64))
			timestamp := int64(eventMsg["timestamp"].(float64))

			// 解析事件体为map
			var eventBody map[string]interface{}
			if err := json.Unmarshal([]byte(eventData), &eventBody); err != nil {
				if err := msg.Nak(); err != nil {
					s.logger.Error("Failed to Nak message", "error", err)
				}
				continue
			}

			// 构建DomainEvent对象
			domainEvent := &entity.BaseEvent{
				Id:            utils.ToString(eventBody["id"]),
				AggregateID:   aggregateID,
				EventType:     eventType,
				AggregateType: utils.ToString(eventBody["aggregateType"]),
				Payload:       eventBody,
				Version:       version,
				Timestamp:     time.Unix(timestamp, 0),
			}

			events = append(events, domainEvent)
		}

		currentVersion++
		if err := msg.Ack(); err != nil {
			s.logger.Error("Failed to Ack message", "error", err)
		}
	}

	return events, nil
}

// GetEventsByType 按事件类型获取事件
func (s *NATSEventStore) GetEventsByType(ctx context.Context, eventType string) ([]entity.DomainEvent, error) {
	// 创建通配符消费者
	consumer, err := s.js.CreateOrUpdateConsumer(ctx, s.streamName, jetstream.ConsumerConfig{
		Durable:       fmt.Sprintf("consumer-type-%s", eventType),
		FilterSubject: fmt.Sprintf("%s.*", s.eventSubject),
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	messages, err := consumer.Fetch(1000)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	var events []entity.DomainEvent
	for msg := range messages.Messages() {
		var eventMsg map[string]interface{}
		if err := json.Unmarshal(msg.Data(), &eventMsg); err != nil {
			if err := msg.Nak(); err != nil {
				s.logger.Error("Failed to Nak message", "error", err)
			}
			continue
		}

		if eventMsg["event_type"].(string) == eventType {
			// 解析事件数据
			eventData := eventMsg["event_data"].(string)
			aggregateID := eventMsg["aggregate_id"].(string)
			version := int(eventMsg["version"].(float64))
			timestamp := int64(eventMsg["timestamp"].(float64))

			// 解析事件体为map
			var eventBody map[string]interface{}
			if err := json.Unmarshal([]byte(eventData), &eventBody); err != nil {
				if err := msg.Nak(); err != nil {
					s.logger.Error("Failed to Nak message", "error", err)
				}
				continue
			}

			// 构建DomainEvent对象
			domainEvent := &entity.BaseEvent{
				Id:            utils.ToString(eventMsg["id"]),
				AggregateID:   aggregateID,
				EventType:     eventType,
				AggregateType: utils.ToString(eventBody["aggregateType"]),
				Payload:       eventBody,
				Version:       version,
				Timestamp:     time.Unix(timestamp, 0),
			}

			events = append(events, domainEvent)
		}
		if err := msg.Ack(); err != nil {
			s.logger.Error("Failed to Ack message", "error", err)
		}
	}

	return events, nil
}

// GetEventsByTimeRange 按时间范围获取事件
func (s *NATSEventStore) GetEventsByTimeRange(ctx context.Context, aggregateID string, fromTime, toTime time.Time) ([]entity.DomainEvent, error) {
	// 使用JetStream的时间戳过滤
	subject := fmt.Sprintf("%s.%s", s.eventSubject, aggregateID)

	consumer, err := s.js.CreateOrUpdateConsumer(ctx, s.streamName, jetstream.ConsumerConfig{
		Durable:       fmt.Sprintf("consumer-time-%s", aggregateID),
		FilterSubject: subject,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	messages, err := consumer.Fetch(1000)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	var events []entity.DomainEvent
	fromUnix := fromTime.Unix()
	toUnix := toTime.Unix()

	for msg := range messages.Messages() {
		var eventMsg map[string]interface{}
		if err := json.Unmarshal(msg.Data(), &eventMsg); err != nil {
			if err := msg.Nak(); err != nil {
				s.logger.Error("Failed to Nak message", "error", err)
			}
			continue
		}

		timestamp := int64(eventMsg["timestamp"].(float64))
		if timestamp >= fromUnix && timestamp <= toUnix {
			// 解析事件数据
			eventData := eventMsg["event_data"].(string)
			eventType := eventMsg["event_type"].(string)
			version := int(eventMsg["version"].(float64))

			// 解析事件体为map
			var eventBody map[string]interface{}
			if err := json.Unmarshal([]byte(eventData), &eventBody); err != nil {
				if err := msg.Nak(); err != nil {
					s.logger.Error("Failed to Nak message", "error", err)
				}
				continue
			}

			// 构建DomainEvent对象
			domainEvent := &entity.BaseEvent{
				Id:            utils.ToString(eventBody["id"]),
				AggregateID:   aggregateID,
				EventType:     eventType,
				AggregateType: utils.ToString(eventBody["aggregateType"]),
				Payload:       eventBody,
				Version:       version,
				Timestamp:     time.Unix(timestamp, 0),
			}

			events = append(events, domainEvent)
		}
		if err := msg.Ack(); err != nil {
			s.logger.Error("Failed to Ack message", "error", err)
		}
	}

	return events, nil
}

// GetEventsWithPagination 分页获取事件
func (s *NATSEventStore) GetEventsWithPagination(ctx context.Context, aggregateID string, limit, offset int) ([]entity.DomainEvent, error) {
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

	if offset >= len(allEvents) {
		return []entity.DomainEvent{}, nil
	}

	end := offset + limit
	if end > len(allEvents) {
		end = len(allEvents)
	}

	return allEvents[offset:end], nil
}

// GetAggregateVersion 获取聚合版本
func (s *NATSEventStore) GetAggregateVersion(ctx context.Context, aggregateID string) (int, error) {
	versionKey := s.versionKey(aggregateID)

	versionData, err := s.bucket.Get(ctx, versionKey)
	if err != nil {
		if err == jetstream.ErrKeyNotFound {
			return 0, nil // 没有事件，版本为0
		}
		return 0, fmt.Errorf("failed to get version: %w", err)
	}

	version, err := strconv.Atoi(string(versionData.Value()))
	if err != nil {
		return 0, fmt.Errorf("invalid version format: %w", err)
	}

	return version, nil
}

// SaveSnapshot 保存快照
func (s *NATSEventStore) SaveSnapshot(ctx context.Context, aggregateID string, snapshot interface{}, version int) error {
	snapshotData, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	snapshotKey := s.snapshotKey(aggregateID)

	// 创建快照数据
	snapshotMsg := map[string]interface{}{
		"aggregate_id":  aggregateID,
		"snapshot_data": string(snapshotData),
		"version":       version,
		"created_at":    time.Now().Unix(),
	}

	snapshotMsgData, err := json.Marshal(snapshotMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot message: %w", err)
	}

	// 保存到Key-Value存储
	_, err = s.bucket.Put(ctx, snapshotKey, snapshotMsgData)
	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	// 发布快照事件
	subject := fmt.Sprintf("%s.%s", s.snapshotSubject, aggregateID)
	_, err = s.js.Publish(ctx, subject, snapshotMsgData)
	if err != nil {
		return fmt.Errorf("failed to publish snapshot: %w", err)
	}

	return nil
}

// Load 从事件流加载聚合根状态
func (s *NATSEventStore) Load(ctx context.Context, aggregateID string, aggregate entity.AggregateRoot) error {
	// 尝试从快照恢复
	snapshot, snapshotVersion, err := s.GetLatestSnapshot(ctx, aggregateID)
	if err != nil && err != errs.ErrSnapshotNotFound {
		return fmt.Errorf("failed to get latest snapshot: %w", err)
	}

	var events []entity.DomainEvent
	var errEvent error

	// 根据快照状态决定获取全部事件或增量事件
	if snapshot != nil {
		// 如果有快照，调用聚合根的LoadFromSnapshot方法恢复状态
		if err := aggregate.LoadFromSnapshot(snapshot); err != nil {
			return fmt.Errorf("failed to load from snapshot: %w", err)
		}

		// 获取快照版本之后的事件
		if events, errEvent = s.GetEventsFromVersion(ctx, aggregateID, snapshotVersion+1); errEvent != nil {
			return fmt.Errorf("failed to get events from version: %w", errEvent)
		}
	} else {
		// 如果没有快照，获取所有事件
		if events, errEvent = s.GetEvents(ctx, aggregateID); errEvent != nil {
			return fmt.Errorf("failed to get events: %w", errEvent)
		}
	}

	// 应用事件到聚合根
	if err := aggregate.LoadFromEvents(events); err != nil {
		return fmt.Errorf("failed to load from events: %w", err)
	}

	return nil
}

// GetLatestSnapshot 获取最新快照
func (s *NATSEventStore) GetLatestSnapshot(ctx context.Context, aggregateID string) (interface{}, int, error) {
	snapshotKey := s.snapshotKey(aggregateID)

	snapshotData, err := s.bucket.Get(ctx, snapshotKey)
	if err != nil {
		if err == jetstream.ErrKeyNotFound {
			return nil, 0, nil // 没有快照
		}
		return nil, 0, fmt.Errorf("failed to get snapshot: %w", err)
	}

	var snapshotMsg map[string]interface{}
	if err := json.Unmarshal(snapshotData.Value(), &snapshotMsg); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	var snapshot interface{}
	if err := json.Unmarshal([]byte(snapshotMsg["snapshot_data"].(string)), &snapshot); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal snapshot data: %w", err)
	}

	version := int(snapshotMsg["version"].(float64))
	return snapshot, version, nil
}

// SaveEventsBatch 批量保存事件
func (s *NATSEventStore) SaveEventsBatch(ctx context.Context, events map[string][]entity.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}

	// 使用事务批量保存
	for aggregateID, eventList := range events {
		if err := s.SaveEvents(ctx, aggregateID, eventList, -1); err != nil {
			return fmt.Errorf("failed to save events for aggregate %s: %w", aggregateID, err)
		}
	}

	return nil
}

// 键生成方法
func (s *NATSEventStore) versionKey(aggregateID string) string {
	return fmt.Sprintf("version:%s", aggregateID)
}

func (s *NATSEventStore) snapshotKey(aggregateID string) string {
	return fmt.Sprintf("snapshot:%s", aggregateID)
}

// Close 关闭连接
func (s *NATSEventStore) Close() error {
	if s.conn != nil {
		s.conn.Close()
	}
	return nil
}
