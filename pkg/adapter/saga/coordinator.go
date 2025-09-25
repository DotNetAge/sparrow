package saga

import (
	"context"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/persistence/repo"
	"github.com/DotNetAge/sparrow/pkg/usecase"
)

// Coordinator 为向后兼容提供的简化包装器
// 位于整洁架构的最外层 - 接口适配器层

type Coordinator struct {
	usecase *usecase.SagaService
}

// NewCoordinator 创建协调器实例
// 使用默认的内存存储实现
func NewCoordinator() *Coordinator {
	repo := repo.NewMemoryRepository[*entity.Transaction]()
	uc := usecase.NewSagaService(repo)
	return &Coordinator{usecase: uc}
}

// Register 注册事务处理器和补偿器
func (c *Coordinator) Register(name string, handler func(context.Context, map[string]interface{}) error, compensator func(context.Context, map[string]interface{}) error) {
	c.usecase.RegisterHandler(name, handler)
	if compensator != nil {
		c.usecase.RegisterCompensator(name, compensator)
	}
}

// Execute 执行事务
func (c *Coordinator) Execute(ctx context.Context, name string, steps []entity.Step) (*entity.Transaction, error) {
	return c.usecase.ExecuteTransaction(ctx, name, steps)
}

// GetTransaction 获取事务状态
func (c *Coordinator) GetTransaction(ctx context.Context, id string) (*entity.Transaction, error) {
	return c.usecase.GetTransaction(ctx, id)
}

// Step 类型别名，保持向后兼容
type Step = entity.Step

// Transaction 类型别名，保持向后兼容
type Transaction = entity.Transaction

// Status 常量，保持向后兼容
const (
	StatusPending     = entity.StatusPending
	StatusRunning     = entity.StatusRunning
	StatusCompleted   = entity.StatusCompleted
	StatusFailed      = entity.StatusFailed
	StatusCompensated = entity.StatusCompensated
)

// NewTransaction 创建事务的工厂函数
func NewTransaction(name string, steps []Step) *Transaction {
	return entity.NewTransaction(name, steps)
}
