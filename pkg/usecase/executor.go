package usecase

// Executor 命令/查询执行器接口
// 统一处理业务命令，实现业务对象.Exec(业务意图)的设计模式
type Executor interface {
	Exec(action interface{}) (interface{}, error)
}