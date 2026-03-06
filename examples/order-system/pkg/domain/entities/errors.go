package entities

// OrderStatusError订单状态错误
type OrderStatusError struct {
	CurrentStatus  OrderStatus
	ExpectedStatus interface{} //可以是OrderStatus或字符串描述
}

func (e *OrderStatusError) Error() string {
	return "invalid order status: current=" + string(e.CurrentStatus) + ", expected=" + e.expectedStatusString()
}

func (e *OrderStatusError) expectedStatusString() string {
	if s, ok := e.ExpectedStatus.(OrderStatus); ok {
		return string(s)
	}
	return e.ExpectedStatus.(string)
}
