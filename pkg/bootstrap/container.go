package bootstrap

import (
	"fmt"
	"reflect"
)

// key 作为 providers map 的键，包含类型和名称
type key struct {
	t    reflect.Type
	name string
}

// provider 定义了如何创建一个实例
type provider struct {
	fn       reflect.Value
	instance reflect.Value // 缓存的实例（用于单例）
}

// Container 是一个支持匿名和具名依赖的 IoC 容器
type Container struct {
	providers map[key]provider
}

// NewContainer 创建一个新的 IoC 容器
func NewContainer() *Container {
	return &Container{
		providers: make(map[key]provider),
	}
}

// Register 注册一个匿名的依赖提供者。
// 它实际上是 RegisterNamed 的一个特例，名称为空字符串。
func (c *Container) Register(constructor interface{}) {
	c.RegisterNamed("", constructor)
}

// RegisterNamed 注册一个具名的依赖提供者。
func (c *Container) RegisterNamed(name string, constructor interface{}) {
	t := reflect.TypeOf(constructor)

	// 确保传入的是一个函数
	if t.Kind() != reflect.Func {
		panic("constructor must be a function")
	}

	// 确保函数至少有一个返回值
	if t.NumOut() == 0 {
		panic("constructor must have at least one return value")
	}

	// 我们只关心第一个返回值的类型，用它作为 map 的键的一部分
	resultType := t.Out(0)
	c.providers[key{t: resultType, name: name}] = provider{fn: reflect.ValueOf(constructor)}
}

// Resolve 从容器中获取一个类型的实例。
// 它会首先尝试查找具名依赖，如果找不到，再尝试查找匿名依赖。
func (c *Container) Resolve(t reflect.Type, name string) (reflect.Value, error) {
	// 1. 尝试查找具名实例的缓存
	k := key{t: t, name: name}
	p, ok := c.providers[k]
	if ok && p.instance.IsValid() {
		return p.instance, nil
	}

	// 2. 如果具名的不存在，尝试查找匿名的（名称为空字符串）
	if !ok {
		anonK := key{t: t, name: ""}
		p, ok = c.providers[anonK]
		if !ok {
			return reflect.Value{}, fmt.Errorf("no provider found for type: %s (name: %s)", t, name)
		}
		// 使用找到的匿名 provider，但保留原始的 key (包含名称) 用于缓存
		// 这样，如果一个匿名 provider 被用于解析一个具名依赖，它的实例也会被缓存为具名的，避免冲突
		k = anonK
	}

	// 3. 解析构造函数的参数（依赖）
	args, err := c.resolveDependencies(p.fn.Type())
	if err != nil {
		return reflect.Value{}, err
	}

	// 4. 调用构造函数创建实例
	results := p.fn.Call(args)
	if len(results) == 0 {
		return reflect.Value{}, fmt.Errorf("constructor for %s returned no value", t)
	}
	instance := results[0]

	// 5. 缓存实例（使用最初请求的 key，可能是具名的或匿名的）
	c.providers[k] = provider{fn: p.fn, instance: instance}

	return instance, nil
}

// resolveDependencies 解析一个函数的所有参数，并从容器中获取它们的实例
// 注意：这个版本的解析器只处理匿名依赖。要处理带名称的参数，需要更复杂的逻辑（见下文）。
func (c *Container) resolveDependencies(fnType reflect.Type) ([]reflect.Value, error) {
	args := make([]reflect.Value, fnType.NumIn())
	for i := 0; i < fnType.NumIn(); i++ {
		argType := fnType.In(i)
		// 默认情况下，解析函数参数时不指定名称，即查找匿名依赖
		argValue, err := c.Resolve(argType, "")
		if err != nil {
			return nil, fmt.Errorf("failed to resolve dependency %d (%s): %w", i, argType, err)
		}
		args[i] = argValue
	}
	return args, nil
}

// 辅助函数，用于更方便地按类型和名称解析
func (c *Container) ResolveByName(name string, out interface{}) error {
	outVal := reflect.ValueOf(out)
	if outVal.Kind() != reflect.Ptr || outVal.IsNil() {
		return fmt.Errorf("out must be a non-nil pointer")
	}
	t := outVal.Type().Elem()
	val, err := c.Resolve(t, name)
	if err != nil {
		return err
	}
	outVal.Elem().Set(val)
	return nil
}

// ResolveInstance 保持不变，用于向后兼容，它只解析匿名依赖
func (c *Container) ResolveInstance(out interface{}) error {
	return c.ResolveByName("", out)
}
