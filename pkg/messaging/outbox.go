package messaging

import (
	"context"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// Outbox 事件发件箱：发布端幂等的落点（P2 接口留位）。
//
// 解决的问题：当前 "Save 与 Publish 两次独立写"（命令管线第 5 步先存聚合再发事件）存在一致性缺口——
//   1. Save 成功、Publish 失败 → 事件丢失；
//   2. 重试 Publish → 事件重复发布（订阅端幂等 P0-3 可兜底，但发布流仍会重复）。
//
// 设计定位：
//   1. Append：业务事务内把事件写入 Outbox（与业务数据同一事务提交，保证不丢）；
//   2. Dispatch：后台投递器异步把 Outbox 中的事件发布到事件流，发布成功后标记已投递；
//   3. 至少一次发布（Dispatch 失败可重试）＋ 订阅端幂等（P0-3）＝ 端到端"恰好一次"。
//
// 实现说明：本文件仅定义接口骨架，具体实现（存储适配器/事务钩子/投递循环/去重键）
// 在 P2 后续随命令管线落地时实现。
type Outbox interface {
	// Append 在业务事务内登记一条待发布事件（由事务钩子调用，与业务数据同事务提交）
	Append(ctx context.Context, event entity.DomainEvent) error

	// Dispatch 投递一批待发布事件到事件流（后台循环调用），返回成功投递的条数。
	// 投递成功的条目标记为已处理，失败条目保留待下次重试。
	Dispatch(ctx context.Context, publisher StreamPublisher, batch int) (int, error)

	// PendingCount 返回当前待投递事件的数量（可观测/管理接口）
	PendingCount(ctx context.Context) (int, error)
}
