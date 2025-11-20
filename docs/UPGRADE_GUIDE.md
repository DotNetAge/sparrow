# 混合任务调度器升级方案

## 概述

本方案提供从现有内存任务调度器升级到混合任务调度器的完整解决方案，支持并发、顺序、流水线三种执行策略，完全向后兼容现有代码。

## 升级方案

### 1. 基础升级（推荐）

**原有代码：**
```go
app := bootstrap.NewApp(
    bootstrap.Tasks(),
    // 其他配置...
)
```

**升级后代码：**
```go
app := bootstrap.NewApp(
    bootstrap.HybridTasks(),  // 一行代码完成升级
    // 其他配置...
)
```

### 2. 自定义配置升级

```go
app := bootstrap.NewApp(
    bootstrap.HybridTasks(
        tasks.WithHybridWorkerCount(10, 2, 2),      // 自定义worker数量
        tasks.WithHybridMaxConcurrentTasks(50),     // 自定义最大并发数
    ),
    // 其他配置...
)
```

### 3. 高级配置升级

```go
// 定义任务策略
taskPolicies := map[string]tasks.TaskExecutionPolicy{
    "data-analysis":    tasks.PolicyConcurrent,
    "file-write":       tasks.PolicySequential,
    "data-pipeline":    tasks.PolicyPipeline,
}

app := bootstrap.NewApp(
    bootstrap.AdvancedTasks(
        10, 2, 2, 50,  // 并发worker, 顺序worker, 流水线worker, 最大并发数
        taskPolicies,   // 任务策略映射
    ),
    // 其他配置...
)
```

## 兼容性保证

### 1. 接口兼容性
- ✅ 完全实现 `tasks.TaskScheduler` 接口
- ✅ 完全实现 `usecase.Startable` 接口  
- ✅ 完全实现 `usecase.GracefulClose` 接口
- ✅ 保持所有现有方法签名不变

### 2. 行为兼容性
- ✅ 现有任务提交代码无需修改
- ✅ 现有任务处理逻辑保持不变
- ✅ App生命周期管理机制完全兼容
- ✅ 优雅关闭机制完全兼容

### 3. 配置兼容性
- ✅ 默认配置提供合理的性能参数
- ✅ 支持渐进式配置升级
- ✅ 支持运行时策略注册

## 升级优势

### 1. 性能提升
- **并发执行**：提升CPU密集型任务处理能力
- **顺序执行**：保证资源访问安全
- **流水线执行**：优化数据处理流程

### 2. 功能增强
- **智能调度**：根据任务类型自动选择执行策略
- **统计监控**：提供详细的执行统计信息
- **灵活配置**：支持多种配置方式

### 3. 运维友好
- **统一管理**：通过App统一管理生命周期
- **错误处理**：完善的错误处理和重试机制
- **监控支持**：内置状态监控和统计

## 实施步骤

### 步骤1：代码更新
在 `pkg/bootstrap/opts.go` 中已添加新的Option函数：
- `HybridTasks()` - 基础混合调度器
- `AdvancedTasks()` - 高级配置调度器

### 步骤2：客户端升级
客户端只需修改一行代码即可完成升级：

```go
// 从
bootstrap.Tasks()

// 改为
bootstrap.HybridTasks()
```

### 步骤3：测试验证
- 运行现有测试用例验证兼容性
- 添加混合调度器特性测试
- 性能基准测试对比

### 步骤4：渐进部署
- 开发环境验证
- 测试环境验证  
- 生产环境灰度发布

## 风险评估

### 低风险
- ✅ 接口完全兼容
- ✅ 行为保持一致
- ✅ 配置有合理默认值

### 缓解措施
- 📋 提供回滚方案（保留原Tasks()函数）
- 📋 完整的测试覆盖
- 📋 渐进式部署策略

## 监控指标

升级后可监控以下指标：
- 各策略任务执行数量
- 任务平均执行时间
- 并发度使用情况
- 错误率和重试次数

## 总结

通过本升级方案，您可以：

1. **零风险升级**：一行代码完成，完全向后兼容
2. **性能显著提升**：多策略执行提升整体性能
3. **功能大幅增强**：获得智能调度和监控能力
4. **运维更加友好**：统一的生命周期管理

这是一个真正"一次升级，长期受益"的方案。