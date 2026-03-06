package repositories
package repositories

import (
    "context"
    "github.com/DotNetAge/sparrow/examples/order-system/pkg/domain/entities"
    "github.com/DotNetAge/sparrow/pkg/usecase"
)

// OrderRepository订单仓储接口
type OrderRepository interface {
    usecase.Repository[*entities.Order]
    
    // FindByCustomerID根据客户ID查找订单
    FindByCustomerID(ctx context.Context, customerID string) ([]*entities.Order, error)
    
    // FindByStatus根据状态查找订单
    FindByStatus(ctx context.Context, status entities.OrderStatus) ([]*entities.Order, error)
}