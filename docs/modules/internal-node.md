# internal/node 模块文档

## 1. 模块功能概述

`internal/node` 提供节点（Node）生命周期与组件管理：

- **节点抽象**：Node 实现 `iface.INode`，持有 Member 信息、序列化器、组件管理器，并对外暴露 System、Cluster。
- **启动流程**：初始化 profile → 加载 node 配置到 Member → 注册并启动组件（Logger、Cluster、System 及用户组件）→ 在集群中 Register → 阻塞等待 SIGINT/SIGQUIT/SIGKILL/SIGTERM。
- **关闭流程**：收到信号后依次 Stop 所有组件（逆序），再调用 `grs.Shutdown` 等待 goroutine 池等收尾。
- **子包 component**：提供可挂载到 Node 的 Logger、Cluster、System 组件，与 profile、actor、cluster、discovery 集成。

## 2. 接口文档

### 2.1 Node（实现 iface.INode）

| 函数/方法 | 说明 |
|-----------|------|
| `New(path string) *Node` | 创建节点，path 为配置文件路径；Member 为空，serializer 默认 Json，组件管理器新建 |
| `Info() *Member` | 返回节点 Member |
| `System() iface.ISystem` | 从组件中取名为 `SystemName` 的组件并断言为 ISystem |
| `Cluster() cluster.ICluster` | 从组件中取名为 `ClusterName` 的组件并断言为 ICluster |
| `SetSerializer(ser lib.ISerializer)` / `Serializer() lib.ISerializer` | 序列化器 |
| `SetPanicHook(hook func(entry zapcore.Entry))` | panic 时回调（如写日志） |
| `Startup(comps ...component.IComponent[INode]) error` | 启动节点：Init profile、加载 node 配置、注册 Logger+Cluster+System+comps、Start 所有组件、集群 Register、阻塞等信号后 shutdown |

### 2.2 内部 shutdown

| 方法 | 说明 |
|------|------|
| `shutdown() error` | 停止所有组件，然后 `grs.Shutdown(timeout)` |

### 2.3 internal/node/component

| 组件 | 名称 | 说明 |
|------|------|------|
| **Logger** | 由 NewLogger 返回 | 从 profile 读 logger 配置并初始化 glog，可选 panicHook |
| **Cluster** | `ClusterName` ("cluster") | 非单节点时从 profile 读 cluster 配置，创建 cluster.Cluster，Run、Subscribe(nodeId, self)；Stop 时 Deregister 并 Shutdown |
| **System** | `SystemName` ("system") | 单节点用 actor.NewSystem，否则 actor.NewClusterSystem(node.GetID(), serializer, node.Cluster())；Stop 时 ISystem.Shutdown() |

接口依赖：`component.IComponent[iface.INode]`（Init、Start、Stop、Name）。

## 3. 设计结构（协程模型与 struct 关系）

### 3.1 协程模型

- **主 goroutine**：执行 `Startup`，在 `signal.Notify` 后阻塞于 `<-sigChan`，收到信号后调用 `shutdown()` 并返回。
- **组件**：由各组件自行决定是否起 goroutine（如 Cluster 内部 MQ、Discovery 的 watcher、Gate 的 listener 等）。
- **grs.Shutdown**：在 shutdown 中调用，用于等待全局 goroutine 池等收尾（超时 30s）。

### 3.2 Struct 关系

```
Node
  ├── *iface.Member
  ├── component.IManager[iface.INode]  (Manager 持有各 IComponent[INode])
  ├── path string
  ├── serializer lib.ISerializer
  └── panicHook func(zapcore.Entry)

Startup 注册的默认组件顺序：
  Logger → Cluster → System → comps...
```

- **Node** 不直接持有 System/Cluster 实例，通过 `GetComponent(SystemName/ClusterName)` 获取并断言。
- **Cluster 组件**：实现 `cluster.ICluster` 与 `messageQue.ISubscriber`，Subscribe 本节点 nodeId，OnMessage 时反序列化为 ActorMessage，根据 Async 走 Send 或 Call，Call 时序列化 Response 并 response(data)。

### 3.3 依赖

- `internal/iface`、`internal/profile`、`internal/actor`、`internal/pb`
- `internal/node/component`（Logger、Cluster、System）
- `pkg/cluster`、`pkg/discovery`、`pkg/glog`、`pkg/lib`、`pkg/lib/component`、`pkg/lib/grs`、`pkg/lib/xerror`
