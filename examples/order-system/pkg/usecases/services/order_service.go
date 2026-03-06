package services

import (
    "context"
    "github.com/DotNetAge/sparrow/examples/order-system/pkg/domain/entities"
    "github.com/DotNetAge/sparrow/examples/order-system/pkg/usecases/commands"
    "github.com/DotNetAge/sparrow/examples/order-system/pkg/usecases/queries"
    "github.com/DotNetAge/sparrow/examples/order-system/pkg/usecases/repositories"
    "github.com/DotNetAge/sparrow/pkg/usecase"
    "github.com/google/uuid"
)

// OrderApplicationService订单应用服务
type OrderApplicationService struct {
    orderRepo     repositories.OrderRepository
    eventStore    usecase.EventStore
    eventPublisher usecase.EventPublisher
}

// NewOrderApplicationService创建订单应用服务
func NewOrderApplicationService(
    orderRepo repositories.OrderRepository,
    eventStore usecase.EventStore,
    eventPublisher usecase.EventPublisher,
) *OrderApplicationService {
    return &OrderApplicationService{
        orderRepo:     orderRepo,
        eventStore:    eventStore,
        eventPublisher: eventPublisher,
    }
}

// CreateOrder创建订单
func (s *OrderApplicationService) CreateOrder(ctx context.Context, cmd commands.CreateOrderCommand) (string, error) {
    // 创建地址值对象
    address := entities.Address{
        Street:  cmd.ShippingAddress.Street,
        City:    cmd.ShippingAddress.City,
        State:   cmd.ShippingAddress.State,
        ZipCode: cmd.ShippingAddress.ZipCode,
        Country: cmd.ShippingAddress.Country,
    }
    
    if !address.Validate() {
        return "", &entities.OrderStatusError{CurrentStatus: "", ExpectedStatus: "valid address required"}
    }
    
    // 创建订单项
    var items []entities.OrderItem
    for _, item := range cmd.Items {
        items = append(items, entities.OrderItem{
            ProductID: item.ProductID,
            Name:      item.Name,
            Price:     item.Price,
            Quantity:  item.Quantity,
        })
    }
    
    // 创建订单聚合根
    order := entities.NewOrder(cmd.CustomerID, items, address)
    
    // 保存到事件存储
    if err := s.eventStore.SaveEvents(ctx, order.GetID(), order.GetUncommittedEvents(), 0); err != nil {
        return "", err
    }
    
    // 发布事件
    if err := s.eventPublisher.PublishEvents(ctx, order.GetUncommittedEvents()); err != nil {
        return "", err
    }
    
    //标记事件为已提交
    order.MarkEventsAsCommitted()
    
    return order.GetID(), nil
}

// ConfirmOrder确认订单
func (s *OrderApplicationService) ConfirmOrder(ctx context.Context, cmd commands.ConfirmOrderCommand) error {
    // 从事件存储加载订单
    order := &entities.Order{}
    if err := s.eventStore.Load(ctx, cmd.OrderID, order); err != nil {
        return err
    }
    
    //确认订单
    if err := order.Confirm(); err != nil {
        return err
    }
    
    // 保存事件
    if err := s.eventStore.SaveEvents(ctx, order.GetID(), order.GetUncommittedEvents(), order.GetVersion()-1); err != nil {
        return err
    }
    
    // 发布事件
    if err := s.eventPublisher.PublishEvents(ctx, order.GetUncommittedEvents()); err != nil {
        return err
    }
    
    //标记事件为已提交
    order.MarkEventsAsCommitted()
    
    return nil
}

// CancelOrder取消订单
func (s *OrderApplicationService) CancelOrder(ctx context.Context, cmd commands.CancelOrderCommand) error {
    // 从事件存储加载订单
    order := &entities.Order{}
    if err := s.eventStore.Load(ctx, cmd.OrderID, order); err != nil {
        return err
    }
    
    //取订单
    if err := order.Cancel(); err != nil {
        return err
    }
    
    // 保存事件
    if err := s.eventStore.SaveEvents(ctx, order.GetID(), order.GetUncommittedEvents(), order.GetVersion()-1); err != nil {
        return err
    }
    
    // 发布事件
    if err := s.eventPublisher.PublishEvents(ctx, order.GetUncommittedEvents()); err != nil {
        return err
    }
    
    //标记事件为已提交
    order.MarkEventsAsCommitted()
    
    return nil
}

// ShipOrder发货订单
func (s *OrderApplicationService) ShipOrder(ctx context.Context, cmd commands.ShipOrderCommand) error {
    // 从事件存储加载订单
    order := &entities.Order{}
    if err := s.eventStore.Load(ctx, cmd.OrderID, order); err != nil {
        return err
    }
    
    // 发货
    if err := order.Ship(); err != nil {
        return err
    }
    
    // 保存事件
    if err := s.eventStore.SaveEvents(ctx, order.GetID(), order.GetUncommittedEvents(), order.GetVersion()-1); err != nil {
        return err
    }
    
    // 发布事件
    if err := s.eventPublisher.PublishEvents(ctx, order.GetUncommittedEvents()); err != nil {
        return err
    }
    
    //标记事件为已提交
    order.MarkEventsAsCommitted()
    
    return nil
}

// DeliverOrder确认收货
func (s *OrderApplicationService) DeliverOrder(ctx context.Context, cmd commands.DeliverOrderCommand) error {
    // 从事件存储加载订单
    order := &entities.Order{}
    if err := s.eventStore.Load(ctx, cmd.OrderID, order); err != nil {
        return err
    }
    
    //确认收货
    if err := order.Deliver(); err != nil {
        return err
    }
    
    // 保存事件
    if err := s.eventStore.SaveEvents(ctx, order.GetID(), order.GetUncommittedEvents(), order.GetVersion()-1); err != nil {
        return err
    }
    
    // 发布事件
    if err := s.eventPublisher.PublishEvents(ctx, order.GetUncommittedEvents()); err != nil {
        return err
    }
    
    //标记事件为已提交
    order.MarkEventsAsCommitted()
    
    return nil
}

// GetOrder获取订单
func (s *OrderApplicationService) GetOrder(ctx context.Context, query queries.GetOrderQuery) (*queries.OrderViewModel, error) {
    // 从仓储获取订单
    order, err := s.orderRepo.Get(ctx, query.OrderID)
    if err != nil {
        return nil, err
    }
    
    //为视图模型
    viewModel := &queries.OrderViewModel{
        ID:         order.GetID(),
        CustomerID: order.CustomerID,
        Items:      convertItems(order.Items),
        TotalPrice: order.TotalPrice,
        Status:     string(order.Status),
        ShippingAddress: queries.Address{
            Street:  order.ShippingAddress.Street,
            City:    order.ShippingAddress.City,
            State:   order.ShippingAddress.State,
            ZipCode: order.ShippingAddress.ZipCode,
            Country: order.ShippingAddress.Country,
        },
        CreatedAt: order.GetCreatedAt().Format("2006-01-02 15:04:05"),
        UpdatedAt: order.GetUpdatedAt().Format("2006-01-02 15:04:05"),
    }
    
    return viewModel, nil
}

// ListOrders列出订单
func (s *OrderApplicationService) ListOrders(ctx context.Context, query queries.ListOrdersQuery) ([]*queries.OrderViewModel, error) {
    // TODO: 实现分页查询逻辑
    var orders []*entities.Order
    var err error
    
    if query.CustomerID != "" {
        orders, err = s.orderRepo.FindByCustomerID(ctx, query.CustomerID)
    } else if query.Status != "" {
        orders, err = s.orderRepo.FindByStatus(ctx, entities.OrderStatus(query.Status))
    } else {
        // 获取所有订单
        orders, err = s.orderRepo.List(ctx, nil)
    }
    
    if err != nil {
        return nil, err
    }
    
    //为视图模型
    var viewModels []*queries.OrderViewModel
    for _, order := range orders {
        viewModel := &queries.OrderViewModel{
            ID:         order.GetID(),
            CustomerID: order.CustomerID,
            Items:      convertItems(order.Items),
            TotalPrice: order.TotalPrice,
            Status:     string(order.Status),
            ShippingAddress: queries.Address{
                Street:  order.ShippingAddress.Street,
                City:    order.ShippingAddress.City,
                State:   order.ShippingAddress.State,
                ZipCode: order.ShippingAddress.ZipCode,
                Country: order.ShippingAddress.Country,
            },
            CreatedAt: order.GetCreatedAt().Format("2006-01-02 15:04:05"),
            UpdatedAt: order.GetUpdatedAt().Format("2006-01-02 15:04:05"),
        }
        viewModels = append(viewModels, viewModel)
    }
    
    return viewModels, nil
}

// convertItems转换订单项
func convertItems(items []entities.OrderItem) []queries.OrderItem {
    var result []queries.OrderItem
    for _, item := range items {
        result = append(result, queries.OrderItem{
            ProductID: item.ProductID,
            Name:      item.Name,
            Price:     item.Price,
            Quantity:  item.Quantity,
        })
    }
    return result
}package services
