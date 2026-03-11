# pkg/cluster 模块文档

## 1. 模块功能概述

`pkg/cluster` 提供集群通信与成员管理：

- **ICluster**：封装服务发现（IDiscovery）与消息队列（IMessageQue）；提供 Run、Subscribe(nodeId, subscriber)、Send(nodeId, message)、Call(nodeId, message, timeout)、Register/Deregister/Update、Select(name, strategy)、GetById/GetByKind/GetByTag/GetAll、Watch/Unwatch、Shutdown。
- **Cluster**：默认实现，单节点模式可跳过启动；Send/Call 前检查成员存在，序列化后 Publish/Request；Select 按 tag 取成员列表并用 RouteStrategy 选一个；Broadcast 向某 tag 所有成员 Send。
- **路由策略**：RouteRandom、RouteRoundRobin(counter)、RouteFirst 等，可自定义 RouteStrategy。

## 2. 接口文档

### 2.1 ICluster

| 方法 | 说明 |
|------|------|
| `Run(ctx) error` | 启动 MQ 与 Discovery |
| `Subscribe(nodeId, subscriber) (ISubscription, error)` | 以 nodeId 为 subject 订阅 |
| `Send(nodeId, message) error` | 序列化后 Publish 到 nodeId subject |
| `Call(nodeId, message, timeout) (data []byte, err error)` | 序列化后 Request，返回反序列化前的 data |
| `Register(member) error` / `Deregister(memberId) error` / `Update(member) error` | 服务发现注册/注销/更新 |
| `Select(name string, strategy RouteStrategy) (nodeId uint64, error)` | 按 tag 取成员，strategy 选一个返回 GetID() |
| `GetById(memberId) *Member` / `GetByKind(kind) map[uint64]*Member` / `GetByTag(tag) []*Member` / `GetAll() map[uint64]*Member` | 查询成员 |
| `Watch(kind, handler)` / `Unwatch(kind, handler)` | 服务变更回调 |
| `Shutdown(ctx) error` | 先 Discovery.Shutdown 再 MQ.Shutdown |

### 2.2 Cluster 构造与配置

| 函数/类型 | 说明 |
|-----------|------|
| `New(config *Config, serializer lib.ISerializer) (*Cluster, error)` | config 可为 nil 用默认；创建 Discovery 与 MQ |
| `Config` | Name、Discovery（discovery.Config）、MessageQueue（messageQue.Config） |
| `DefaultConfig() *Config` | 默认 Type consul / nats |

### 2.3 路由策略

| 类型/函数 | 说明 |
|-----------|------|
| `RouteStrategy` | `func(members []*Member) *Member` |
| `RouteRandom(members)` | 随机一个 |
| `RouteRoundRobin(counter *uint64) RouteStrategy` | 轮询 |
| `RouteFirst(members)` | 第一个 |

### 2.4 扩展方法

| 方法 | 说明 |
|------|------|
| `Broadcast(tag string, message interface{})` | GetByTag(tag)，对每个成员 Send |

错误：`ErrNotFoundMember`。

## 3. 设计结构（协程模型与 struct 关系）

### 3.1 协程模型

- **Run**：MQ.Run 与 IDiscovery.Run 内部可能起 goroutine（订阅、watcher 等）。
- **Send/Call**：在调用方 goroutine 中序列化并 Publish/Request；Call 阻塞至超时或收到回复。
- **Subscribe 回调**：由 MQ 的订阅 goroutine 调用 OnMessage，Cluster 的 OnMessage（node 组件实现）里再调 system.Send/Call。

### 3.2 Struct 关系

```
Cluster
  ├── stopper.Stopper
  ├── serializer lib.ISerializer
  ├── discovery.IDiscovery (接口，如 consul)
  ├── mq messageQue.IMessageQue (接口，如 nats)
  └── localInfo *discovery.Member (可选，Register 时用)

Config
  ├── Discovery *dis.Config
  └── MessageQueue *mq.Config
```

- **node 组件**：Cluster 组件实现 ICluster 与 ISubscriber；Subscribe(node.GetID(), self)，收到消息后反序列化为 ActorMessage，根据 Async 走 Send 或 Call，Call 时序列化 Response 并 response(data)。

### 3.3 依赖

- `pkg/discovery`、`pkg/discovery/iface`、`pkg/messageQue`、`pkg/messageQue/iface`
- `pkg/glog`、`pkg/lib`、`pkg/lib/stopper`、`pkg/lib/xerror`
- `github.com/duke-git/lancet/v2/convertor`
