package infrastructures

import (
    "context"
    "sync"
    "github.com/DotNetAge/sparrow/examples/order-system/pkg/domain/entities"
    "github.com/DotNetAge/sparrow/examples/order-system/pkg/usecases/repositories"
    "github.com/DotNetAge/sparrow/pkg/persistence/repo"
)

// InMemoryOrderRepository内存订单仓储实现
type InMemoryOrderRepository struct {
    repo *repo.MemoryRepository[*entities.Order]
    mu   sync.RWMutex
}

// NewInMemoryOrderRepository创建内存订单仓储
func NewInMemoryOrderRepository() *InMemoryOrderRepository {
    return &InMemoryOrderRepository{
        repo: repo.NewMemoryRepository[*entities.Order](),
    }
}

// Save保存订单
func (r *InMemoryOrderRepository) Save(ctx context.Context, order *entities.Order) error {
    return r.repo.Save(ctx, order)
}

// Get获取订单
func (r *InMemoryOrderRepository) Get(ctx context.Context, id string) (*entities.Order, error) {
    return r.repo.Get(ctx, id)
}

// Delete删除订单
func (r *InMemoryOrderRepository) Delete(ctx context.Context, id string) error {
    return r.repo.Delete(ctx, id)
}

// List列出订单
func (r *InMemoryOrderRepository) List(ctx context.Context, query interface{}) ([]*entities.Order, error) {
    return r.repo.List(ctx, query)
}

// FindByCustomerID根据客户ID查找订单
func (r *InMemoryOrderRepository) FindByCustomerID(ctx context.Context, customerID string) ([]*entities.Order, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    // 获取所有订单然后过滤
    orders, err := r.repo.List(ctx, nil)
    if err != nil {
        return nil, err
    }
    
    var result []*entities.Order
    for _, order := range orders {
        if order.CustomerID == customerID {
            result = append(result, order)
        }
    }
    
    return result, nil
}

// FindByStatus根据状态查找订单
func (r *InMemoryOrderRepository) FindByStatus(ctx context.Context, status entities.OrderStatus) ([]*entities.Order, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    // 获取所有订单然后过滤
    orders, err := r.repo.List(ctx, nil)
    if err != nil {
        return nil, err
    }
    
    var result []*entities.Order
    for _, order := range orders {
        if order.Status == status {
            result = append(result, order)
        }
    }
    
    return result, nil
}package repositories
