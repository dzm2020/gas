# pkg/messageQue 模块文档

## 1. 模块功能概述

`pkg/messageQue` 提供消息队列抽象与实现：

- **IMessageQue 接口**：Run、Publish、Request、Subscribe、Shutdown；用于集群节点间异步 Send 与同步 Call。
- **配置驱动**：Config 含 Type（如 "nats"）与 Config map；NewFromConfig 通过 registry 按 Type 创建实现。
- **provider/nats**：init 时注册 "nats"；Client 持连接池（按 subject 固定连接保证顺序）与独立订阅连接；Publish/Request 用池，Subscribe 用 subConn，OnMessage 回调中通过 response 回包。

## 2. 接口文档

### 2.1 iface

| 接口 | 说明 |
|------|------|
| `IMessageQue` | Run(ctx)、Publish(subject, data)、Request(subject, data, timeout)、Subscribe(subject, subscriber)、(ISubscription)、Shutdown(ctx) |
| `ISubscription` | Unsubscribe() error |
| `ISubscriber` | OnMessage(request []byte, response func(data []byte) error) |

### 2.2 messageQue 包

| 类型/函数 | 说明 |
|-----------|------|
| `Config` | Type、Config map[string]interface{} |
| `NewFromConfig(config Config) (IMessageQue, error)` | 从 registry 按 Type 创建 |
| `init.go` | import _ provider/nats 以注册 |

### 2.3 provider/nats

| 类型/方法 | 说明 |
|-----------|------|
| `Config` | Servers 等（viper 解析） |
| `New(cfg *Config) *Client` | 创建 Client |
| `Client` | Run：建池与 subConn；Subscribe：subConn.Subscribe，回调里 subscriber.OnMessage(m.Data, response)；Publish/Request：pool.getConnBySubject；Shutdown：Stop、关 subConn 与 pool |

## 3. 设计结构（协程模型与 struct 关系）

### 3.1 协程模型

- **Run**：连接池与 subConn 建立，可能在调用 goroutine。
- **Subscribe**：NATS 内部在独立 goroutine 收消息并调用 OnMessage；response(data) 即 Respond(data)。
- **Request**：在调用方 goroutine 阻塞直到回复或超时。

### 3.2 Struct 关系

```
Config (Type + Config map)
  -> NewFromConfig -> registry.Get(type) -> creator(Config) -> IMessageQue

nats.Client
  ├── stopper.Stopper
  ├── cfg *Config
  ├── pool *ConnPool (按 subject 取 conn，保证同 subject 顺序)
  └── subConn *nats.Conn (专用于 Subscribe)
```

### 3.3 依赖

- `pkg/messageQue/iface`、`pkg/messageQue/registry`
- `pkg/glog`、`pkg/lib/stopper`、`pkg/lib/xerror`
- `github.com/nats-io/nats.go`、`github.com/spf13/viper`
