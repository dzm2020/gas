# internal/component/db/redis 模块文档

## 1. 模块功能概述

`internal/component/db/redis` 提供 Redis 组件：

- **多实例管理**：通过 id（如 0、1）管理多组 Redis 连接（Client），支持 Get(id)、Add(id, conf)、Has(id)、Range(fn)、Close()。
- **Client**：封装 `redis.UniversalClient`，支持单机/集群等；创建时 Ping 校验。
- **组件生命周期**：作为 Node 组件启动时从 profile 读 "redis" 配置（数组），为每项 Add；并可为所有 Client 加载脚本（loadAllScripts）；Stop 时 Close 全部连接。
- 可选：locker、script_manager 等扩展（见 locker.go、script_manager.go）。

## 2. 接口文档

### 2.1 组件

| 方法 | 说明 |
|------|------|
| `Name() string` | "redis" |
| `Start(ctx, node) error` | profile.Get("redis", &configs)，对每项 Add(i, config)，再 Range 每 Client loadAllScripts |
| `Stop(ctx) error` | Close() |

### 2.2 全局管理

| 函数 | 说明 |
|------|------|
| `Get(id int) *Client` | 按 id 取 Client |
| `Add(id int, conf *Config) error` | 已存在直接返回 nil；否则 newClient 并 Store |
| `Has(id int) bool` | 是否存在 |
| `Range(fn func(*Client))` | 遍历所有 Client |
| `Close()` | 遍历并 Close、Delete |

### 2.3 Config 与 Client

| 类型 | 说明 |
|------|------|
| `Config` | Address []string、Password、PoolSize（yaml/json tag） |
| `Client` | 嵌 redis.UniversalClient，持 conf；内部 newClient 时 Ping |

### 2.4 辅助

| 函数 | 说明 |
|------|------|
| `IsNil(err error) bool` | errors.Is(err, redis.Nil) |
| `Error(err error) bool` | 非 Nil 且 err != nil |

## 3. 设计结构（协程模型与 struct 关系）

### 3.1 协程模型

- Redis 客户端内部使用连接池，可能有多 goroutine 访问；UniversalClient 自身线程安全。
- 无包内常驻 goroutine；Range/Close 在调用方 goroutine 执行。

### 3.2 Struct 关系

```
Component (BaseComponent[INode])
  └── Start: profile.Get("redis", &configs) -> Add(i, config) -> Range(loadAllScripts)
  └── Stop: Close()

全局 sync.Map: id -> *Client
  └── Add/Get/Has/Range/Close
```

### 3.3 依赖

- `github.com/go-redis/redis/v8`
- `internal/profile`、`internal/iface`、`pkg/lib/component`
