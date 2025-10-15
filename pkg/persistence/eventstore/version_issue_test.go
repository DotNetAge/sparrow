package eventstore

import (
	"context"
	"fmt"
	"testing"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestVersionIssueAfterNineEvents 测试在应用9次事件后第10次存储失败的问题
func TestVersionIssueAfterNineEvents(t *testing.T) {
	// 使用包中已有的测试辅助函数设置测试环境
	store, cleanup := setupTestEventStore(t)
	defer cleanup()

	ctx := context.Background()
	aggregateID := "test-aggregate-version-issue"

	// 生成10个不同的测试事件
	var events []entity.DomainEvent
	for i := 0; i < 20; i++ {
		event := NewTestRoleCreated(
			aggregateID,
			fmt.Sprintf("role-%d", i),
			fmt.Sprintf("Role %d", i),
		)
		// 为了确保每个事件都是唯一的，修改事件ID
		event.Id = uuid.New().String()
		events = append(events, event)
	}

	// 逐个应用事件，模拟真实场景中的多次保存
	expectedVersion := 0
	for i := 0; i < 20; i++ {
		// 每次只保存一个事件
		singleEvent := []entity.DomainEvent{events[i]}
		t.Logf("尝试保存第 %d 个事件，期望版本: %d", i+1, expectedVersion)
		err := store.SaveEvents(ctx, aggregateID, singleEvent, expectedVersion)
		assert.NoError(t, err, "保存第 %d 个事件失败", i+1)
		expectedVersion++
	}

	// 验证最终版本号是否为10
	finalVersion, err := store.GetAggregateVersion(ctx, aggregateID)
	assert.NoError(t, err)
	assert.Equal(t, 20, finalVersion, "最终版本号应该是10")

	// 尝试再保存一个事件，验证是否还能继续工作
	additionalEvent := NewTestRoleCreated(aggregateID, "role-10", "Role 10")
	additionalEvent.Id = uuid.New().String()
	err = store.SaveEvents(ctx, aggregateID, []entity.DomainEvent{additionalEvent}, 10)
	assert.NoError(t, err, "保存第11个事件失败")

	// 再次验证最终版本号
	finalVersionAfterAdditional, err := store.GetAggregateVersion(ctx, aggregateID)
	assert.NoError(t, err)
	assert.Equal(t, 11, finalVersionAfterAdditional, "最终版本号应该是11")
}

// TestVersionIssueWithBatchSaves 测试使用批处理保存事件时的版本问题
func TestVersionIssueWithBatchSaves(t *testing.T) {
	store, cleanup := setupTestEventStore(t)
	defer cleanup()

	ctx := context.Background()
	aggregateID := "test-aggregate-batch-version-issue"

	// 第一次保存3个事件
	eventsBatch1 := []entity.DomainEvent{
		NewTestRoleCreated(aggregateID, "role-1", "Role 1"),
		NewTestRoleCreated(aggregateID, "role-2", "Role 2"),
		NewTestRoleCreated(aggregateID, "role-3", "Role 3"),
	}
	err := store.SaveEvents(ctx, aggregateID, eventsBatch1, 0)
	assert.NoError(t, err, "保存第一批事件失败")

	// 验证版本号为3
	versionAfterBatch1, err := store.GetAggregateVersion(ctx, aggregateID)
	assert.NoError(t, err)
	assert.Equal(t, 3, versionAfterBatch1, "第一批事件后版本号应该是3")

	// 第二次保存3个事件
	eventsBatch2 := []entity.DomainEvent{
		NewTestRoleCreated(aggregateID, "role-4", "Role 4"),
		NewTestRoleCreated(aggregateID, "role-5", "Role 5"),
		NewTestRoleCreated(aggregateID, "role-6", "Role 6"),
	}
	err = store.SaveEvents(ctx, aggregateID, eventsBatch2, 3)
	assert.NoError(t, err, "保存第二批事件失败")

	// 验证版本号为6
	versionAfterBatch2, err := store.GetAggregateVersion(ctx, aggregateID)
	assert.NoError(t, err)
	assert.Equal(t, 6, versionAfterBatch2, "第二批事件后版本号应该是6")

	// 第三次保存4个事件，测试是否会在达到版本10时失败
	eventsBatch3 := []entity.DomainEvent{
		NewTestRoleCreated(aggregateID, "role-7", "Role 7"),
		NewTestRoleCreated(aggregateID, "role-8", "Role 8"),
		NewTestRoleCreated(aggregateID, "role-9", "Role 9"),
		NewTestRoleCreated(aggregateID, "role-10", "Role 10"),
	}
	err = store.SaveEvents(ctx, aggregateID, eventsBatch3, 6)
	assert.NoError(t, err, "保存第三批事件失败")

	// 验证最终版本号为10
	finalVersion, err := store.GetAggregateVersion(ctx, aggregateID)
	assert.NoError(t, err)
	assert.Equal(t, 10, finalVersion, "最终版本号应该是10")
}

// TestVersionIssueWithAggregate 测试使用聚合根对象时的版本问题
func TestVersionIssueWithAggregate(t *testing.T) {
	store, cleanup := setupTestEventStore(t)
	defer cleanup()

	ctx := context.Background()
	aggregateID := "test-aggregate-obj-version-issue"
	aggregate := NewMockAggregateRoot(aggregateID)

	// 通过聚合根应用10次事件
	for i := 0; i < 10; i++ {
		event := NewTestRoleCreated(
			aggregateID,
			fmt.Sprintf("role-%d", i),
			fmt.Sprintf("Role %d", i),
		)
		event.Id = uuid.New().String()

		// 应用事件到聚合根
		err := aggregate.ApplyEvent(event)
		assert.NoError(t, err, "应用第 %d 个事件到聚合根失败", i+1)

		// 保存事件
		t.Logf("尝试保存第 %d 个事件，期望版本: %d", i+1, i)
		err = store.SaveEvents(ctx, aggregateID, []entity.DomainEvent{event}, i)
		assert.NoError(t, err, "保存第 %d 个事件失败", i+1)
	}

	// 验证聚合根版本号为10
	assert.Equal(t, 10, aggregate.Version, "聚合根版本号应该是10")

	// 验证存储中的版本号为10
	storeVersion, err := store.GetAggregateVersion(ctx, aggregateID)
	assert.NoError(t, err)
	assert.Equal(t, 10, storeVersion, "存储中的版本号应该是10")
}
