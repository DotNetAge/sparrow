太棒了！将这个自定义的 IoC 容器集成到你的项目中是一个非常好的实践。下面我为你整理一份详细的**使用指南**，你可以将其作为项目文档存档，方便自己和团队成员查阅。

---

# 自定义 IoC 容器使用指南

本文档介绍了如何在项目中使用我们自定义的、支持**匿名依赖**和**具名依赖**的 IoC (Inversion of Control) 容器。该容器旨在简化对象的创建和管理，实现组件间的解耦。

## 1. 核心概念

在使用容器之前，请理解以下两个核心概念：

- **Provider (提供者)**：一个普通的 Go 函数，负责创建并返回一个特定类型的实例。这个函数的参数就是它的依赖项。
  - **示例**：`func NewUserService(repo *UserRepository) *UserService` 是一个 `*UserService` 的 Provider。
- **Dependency (依赖)**：一个对象创建过程中所需要的其他对象或值。
  - **匿名依赖 (Anonymous Dependency)**：仅通过类型来标识的依赖。当一个类型在整个应用中只有一个实例时，可以直接使用匿名依赖。
  - **示例**：`*UserRepository` 是一个匿名依赖。*   **具名依赖 (Named Dependency)**：通过“类型 + 唯一名称”来标识的依赖。用于解决**基本类型**（如 `string`, `int`）或**重复强类型**（如两个不同配置的 `*Logger`）带来的歧义。

## 2. 快速上手：一个完整的例子

假设我们有以下业务代码：

```go
// user.go
package myapp

type UserRepository struct{}
func NewUserRepository() *UserRepository { return &UserRepository{} }

type UserService struct {
    Repo *UserRepository
}
func NewUserService(repo *UserRepository) *UserService {
    return &UserService{Repo: repo}
}
```

**使用步骤：**

1.  **创建容器**
```go
import "path/to/your/ioc"

container := ioc.NewContainer()
```

2.  **注册 Provider**
  


```go
// 注册 UserRepository 和 UserService 的创建函数
container.Register(NewUserRepository)
container.Register(NewUserService)
```

3.  **解析实例**

```go
var userService *UserService
err := container.ResolveInstance(&userService)
if err != nil {
    panic(err) // 处理错误
}

// 现在 userService 已经准备好，可以直接使用
// userService.Repo 也已经被自动注入
```

## 3. 高级用法：处理复杂依赖

### 3.1 如何处理基本类型参数（如 `string`, `int`）

当构造函数需要配置项（如数据库地址、端口号）时，使用**具名依赖**。

**场景**：`OrderService` 需要一个 `*DB` 连接和一个 `string` 类型的应用名。

```go
// order.go
type OrderService struct {
    DB      *DB
    AppName string
}
func NewOrderService(db *DB, appName string) *OrderService {
    return &OrderService{DB: db, AppName: appName}
}
```

**解决方案**：

1.  **注册具名的基本类型**
  
```go
// 将配置值注册为具名依赖
container.RegisterNamed("appName", func() string {
    return "MyAwesomeApp"
})

// 假设 DB 已经作为匿名依赖注册
container.Register(func() *DB { return &DB{} })
```

1. **使用闭包作为“适配器”注册最终服务**
   
```go
// 创建一个闭包，它只依赖匿名的 *DB
// 在闭包内部，手动从容器中解析具名的 "appName"
container.Register(func(db *DB) *OrderService {
    var appName string
    if err := container.ResolveByName("appName", &appName); err != nil {
        panic(err)
    }
    return NewOrderService(db, appName)
})
```

1.  **像之前一样解析**

```go
var orderService *OrderService
container.ResolveInstance(&orderService) // 成功！
```

### 3.2 如何处理重复的强类型

当一个构造函数需要多个同类型但不同用途的实例时，也使用**具名依赖**。

**场景**：`ReportService` 需要一个访问日志 `*Logger` 和一个错误日志 `*Logger`。

```go
// report.go
type Logger struct { Level string }
type ReportService struct {
    AccessLogger *Logger
    ErrorLogger  *Logger
}
func NewReportService(accessLogger, errorLogger *Logger) *ReportService {
    return &ReportService{AccessLogger: accessLogger, ErrorLogger: errorLogger}
}
```

**解决方案**：

1.  **注册具名的重复类型**

```go
container.RegisterNamed("access", func() *Logger {
    return &Logger{Level: "INFO"}
})
container.RegisterNamed("error", func() *Logger {
    return &Logger{Level: "ERROR"}
})
```

1.  **使用闭包作为“适配器”注册最终服务**

```go
container.Register(func() *ReportService {
    var accessLog *Logger
    var errorLog *Logger
    // 分别解析两个具名的 Logger
    if err := container.ResolveByName("access", &accessLog); err != nil {
        panic(err)
    }
    if err := container.ResolveByName("error", &errorLog); err != nil {
        panic(err)
    }
    return NewReportService(accessLog, errorLog)
})

```

## 4. API 参考

### `func NewContainer() *Container`
创建一个新的、空的 IoC 容器实例。

### `func (c *Container) Register(constructor interface{})`
注册一个**匿名**的依赖提供者。
*   `constructor` 必须是一个函数。
*   函数的返回值是你想要注册的类型。
*   函数的参数是该类型的依赖（这些依赖也必须在容器中可解析）。

### `func (c *Container) RegisterNamed(name string, constructor interface{})`
注册一个**具名**的依赖提供者。
*   `name` 是一个唯一的字符串，用于区分同类型的不同实例。
*   其他要求同 `Register`。

### `func (c *Container) ResolveInstance(out interface{}) error`
从容器中解析一个**匿名**依赖的实例。
*   `out` 必须是一个指向你想要获取的类型的**非空指针**。
*   容器会递归地创建所有依赖，并注入到最终实例中。
*   如果解析失败（如依赖缺失），返回 `error`。

### `func (c *Container) ResolveByName(name string, out interface{}) error`
从容器中解析一个**具名**依赖的实例。
*   `name` 是你在 `RegisterNamed` 时使用的名称。
*   `out` 必须是一个指向你想要获取的类型的**非空指针**。

## 5. 最佳实践与建议

1.  **优先使用匿名依赖**：对于清晰、唯一的业务对象（如 `*UserRepository`, `*PaymentService`），直接使用 `Register` 和 `ResolveInstance`，代码最简洁。

2.  **用 `struct` 聚合配置**：如果有多个相关的配置项（如 `Host`, `Port`, `Timeout`），强烈建议将它们封装到一个 `Config` 结构体中，然后注册这个结构体的指针 `*Config`。这比注册大量零散的具名基本类型更清晰、更易于管理。

3.  **`main` 函数是唯一的“组合根” (Composition Root)**：将所有 `Register` 调用和最终的 `Resolve` 调用集中在 `main` 函数或专门的 `init` 包中。业务代码本身不应该知道容器的存在，它们只通过构造函数接收依赖。

4.  **处理错误**：`ResolveInstance` 和 `ResolveByName` 会返回 `error`。务必检查并处理这些错误，通常在应用启动时发生的依赖解析错误是致命的，应该 `panic` 或优雅地退出。

5.  **理解单例**：当前的容器实现默认是**单例（Singleton）**模式。即一个类型（或具名类型）只会被创建一次，之后每次 `Resolve` 都会返回同一个实例。这通常是你想要的行为。如果需要原型（每次都创建新实例），则需要修改容器的缓存逻辑。