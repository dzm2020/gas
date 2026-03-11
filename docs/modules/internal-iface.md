# internal/iface 模块文档

## 1. 模块功能概述

`internal/iface` 定义全库共享的接口与部分通用类型，用于：

- **解耦实现**：actor、gate、node 等仅依赖接口，便于测试与替换实现。
- **统一抽象**：Pid、Actor 消息、Session、Node、Member 等在此集中定义，避免循环依赖。
- **编译期校验**：通过 `var _ Interface = (*Impl)(nil)` 保证实现满足接口。

不包含业务逻辑，仅类型与接口定义。

## 2. 接口文档

### 2.1 类型别名与通用类型

| 类型 | 说明 |
|------|------|
| `Pid` | `= pb.Pid`，进程标识（NodeId、ActorId、ActorName） |
| `Member` | `= discovery.Member`，集群成员信息 |

### 2.2 消息与任务

| 接口/类型 | 说明 |
|-----------|------|
| `IMessage` | `Validate() error`，ActorMessage、TaskMessage 实现 |
| `Task` | `func(ctx IContext) error`，投递到进程的任务 |
| `ActorMessage` | 含 `*pb.Message`、`ResponseFunc`；用于 Send/Call |
| `TaskMessage` | 含 `Task`；用于 SubmitTask |
| `ResponseFunc` | `func(data []byte, err error)`，Call 的响应回调 |
| `Response` | 包装 `*pb.Response`，`GetError() error` |

常用构造：`NewActorMessage(from, to, methodName, data)`、`NewTaskMessage(task)`、`NewPid(nodeId, actorId)`、`NewPidWithName(name, nodeId)`、`NewResponse(data, err)`。

### 2.3 进程与系统

| 接口 | 说明 |
|------|------|
| `IMessageInvoker` | `InvokerMessage(message interface{}) error` |
| `IProcess` | `PostMessage(IMessage) error`、`Shutdown() error` |
| `ISystem` | NodeId、NextID、SessionFactory、Serializer、Spawn、Register、Unregister、Named、Unname、SubmitTask、SubmitTaskAndWait、Send、Call、GetProcess、GetAllProcesses、ShutdownProcess、Shutdown |

### 2.4 上下文与 Actor

| 接口 | 说明 |
|------|------|
| `IContext` | ID、Serializer、Named/Unname、GetName、Actor、Message、Process、System、SetCallTimeout、Send、Call、Forward、AfterFunc、Shutdown；继承 IMessageInvoker |
| `IActor` | `OnInit(ctx, params)`、`OnMessage(ctx, msg)`、`OnStop(ctx)` |
| `IRouter` | `Handle(ctx, methodName, session, data)`、`HasRoute(methodName)`、`AutoRegister(actor)` |

### 2.5 Session（actor 侧只依赖接口）

| 接口 | 说明 |
|------|------|
| `ISession` | GetId、Raw、SetString/GetString、SetUint64/GetUint64、SetInt64/GetInt64 |
| `ISessionFactory` | `FromRaw(ctx, raw *pb.Session) ISession`，由 gate 实现，actor 不依赖具体 Session 实现 |

### 2.6 Node 与成员

| 接口 | 说明 |
|------|------|
| `IMember` | GetKind、GetID、GetAddress、GetPort、GetTags、GetMeta |
| `INode` | 继承 IMember；Info、Serializer、SetSerializer、System、Cluster、Startup(comps...)；并实现 `component.IManager[INode]` |

### 2.7 辅助

| 函数 | 说明 |
|------|------|
| `EqualPid(o, other *Pid) bool` | 自定义 Pid 相等判断 |

### 2.8 错误（在 iface 或 message 子包）

- `ErrMessageMethodIsNil`、`ErrTaskMessageIsNil`、`ErrTaskIsNilInMsg`、`ErrMessageTargetIsNil`、`ErrMessageTargetInvalid`、`ErrSyncMessageIsNil`

### 2.9 默认实现

- `Actor` 结构体：OnInit/OnMessage/OnStop 空实现，可嵌入并重写。

## 3. 设计结构（协程模型与 struct 关系）

### 3.1 协程模型

本包无协程，仅接口与类型定义。协程与生命周期由实现包（actor、node、gate 等）决定。

### 3.2 Struct 关系

- **Pid / Message / Session**：来自或关联 `internal/pb`，跨节点传递时由 cluster 与 serializer 序列化。
- **INode**：组合 `*iface.Member` 与 `component.IManager[INode]`，由 `internal/node` 的 `Node` 实现；通过 `GetComponent(system/cluster)` 拿到 ISystem、ICluster。
- **ISessionFactory**：由 gate 的 `session.Factory` 实现，在 System 启动 Gate 时注入，actor 侧通过 `ctx` 和 `FromRaw` 得到 ISession，不依赖 gate 具体类型。

### 3.3 依赖

- `internal/pb`
- `pkg/cluster`（ICluster）
- `pkg/discovery/iface`（Member）
- `pkg/lib`（ISerializer、Timer）
- `pkg/lib/component`（IManager、IComponent）
