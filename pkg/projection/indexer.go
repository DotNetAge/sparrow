package projection

// AggregateIndexer 聚合根索引管理器接口
// 负责维护和查询所有有效聚合根的ID
//
//	使用场景：用于定时投影与全量投影时使用
type AggregateIndexer interface {
	// GetAllAggregateIDs 获取指定聚合类型的所有有效聚合根ID
	// aggregateType：聚合根类型（如"order"、"user"）
	// 返回值：该类型下的所有聚合根ID列表
	GetAllAggregateIDs(aggregateType string) ([]string, error)
}
