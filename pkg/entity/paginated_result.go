package entity

// PaginatedResult 泛型分页查询结果
// 用于支持分页查询的返回格式，Data 属性为泛型集合
type PaginatedResult[T any] struct {
	Data  []T   `json:"data"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}

// NewPaginatedResult 创建新的分页结果实例
func NewPaginatedResult[T any](data []T, total int64, page, size int) *PaginatedResult[T] {
	return &PaginatedResult[T]{
		Data:  data,
		Total: total,
		Page:  page,
		Size:  size,
	}
}
