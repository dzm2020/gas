# pkg/cluster 模块文档

## 1. 概述

`pkg/cluster` 提供集群通信与成员管理，对外以 **ICluster** 接口呈现：封装服务发现与消息队列，提供 Run、订阅/收发、注册与查询、选路、Watch、Shutdown。默认实现为 **Cluster**，内部依赖 pkg/discovery 与 pkg/messageQue，此处仅说明 ICluster 接口。

---

## 2. ICluster 接口

| 方法 | 说明 |
|------|------|
| `Run(ctx context.Context) error` | 启动内部消息队列与服务发现 |
| `Subscribe(nodeId uint64, subscriber messageQue.ISubscriber) (messageQue.ISubscription, error)` | 以 nodeId 为 subject 订阅，收到消息时回调 subscriber.OnMessage |
| `Send(nodeId uint64, message interface{}) error` | 将 message 序列化后发送到 nodeId 对应 subject（单向） |
| `Call(nodeId uint64, message interface{}, timeout time.Duration) ([]byte, error)` | 序列化后请求 nodeId，阻塞至收到回复或超时，返回反序列化前的 data |
| `Register(member *discovery.Member) error` | 注册成员到服务发现 |
| `Deregister(memberId uint64) error` | 从服务发现注销成员 |
| `Update(member *discovery.Member) error` | 更新成员信息 |
| `Select(tag string, strategy RouteStrategy) (uint64, error)` | 按 tag 取成员列表，用 strategy 选一个成员，返回其 GetID()；无成员或选路失败返回 ErrNotFoundMember |
| `GetById(memberId uint64) *discovery.Member` | 按 ID 查询成员 |
| `GetByKind(kind string) map[uint64]*discovery.Member` | 按 Kind 查询成员映射 |
| `GetByTag(tag string) []*discovery.Member` | 按 Tag 查询成员列表 |
| `GetAll() map[uint64]*discovery.Member` | 返回所有成员 |
| `Watch(kind string, handler discovery.ServiceChangeHandler)` | 注册拓扑变更回调 |
| `Unwatch(kind string, handler discovery.ServiceChangeHandler)` | 取消拓扑变更回调 |
| `Shutdown(ctx context.Context) error` | 关闭（先服务发现再消息队列） |

- **RouteStrategy**：`func(members []*discovery.Member) *discovery.Member`，用于 Select 的选路策略；包内提供 RouteRandom、RouteRoundRobin、RouteFirst 等。
- **错误**：选路或未找到成员时返回 `ErrNotFoundMember`。
