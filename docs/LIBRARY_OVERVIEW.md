# GAS 库总览

## 一、库解决的问题

**GAS**（Gateway-Actor-System）是一个面向**分布式游戏/长连接服务**的 Go 框架，主要解决以下问题：

1. **并发与一致性**  
   通过 **Actor 模型** 将状态与逻辑封装在“进程”内，单进程内消息串行处理，避免共享可变状态带来的竞态；同时支持单节点与集群两种模式，集群下跨节点消息通过消息队列与序列化透明完成。

2. **长连接与网关**  
   提供 **Gate** 组件：监听 TCP/UDP/WebSocket，为每条连接创建一个 **Agent**（Actor），连接上的数据经协议解包后投递到对应 Agent 的 Mailbox，由业务通过 **Session** 做响应、推送与关闭，并支持中间件（日志、压缩、加密、限流等）。

3. **节点与集群**  
   **Node** 统一节点生命周期：配置（profile）、组件（Logger、Cluster、System、Gate、Redis 等）的注册与启停、服务发现注册、信号处理与优雅退出。**Cluster** 封装服务发现与消息队列，提供按节点 Send/Call、按 Tag 选节点、广播等能力。

4. **可扩展与可测试**  
   核心能力均通过 **internal/iface** 中的接口抽象，实现可替换、易 mock；组件化设计（pkg/lib/component）便于按需挂载 Gate、Redis 等，并保证启动/停止顺序。

5. **基础设施**  
   提供 **网络层**（pkg/network）、**日志**（pkg/glog）、**配置**（internal/profile）、**序列化**（pkg/lib serializer）、**服务发现**（pkg/discovery）、**消息队列**（pkg/messageQue）等通用能力，并与 Actor/Cluster 集成。

---

## 二、如何接入与使用

### 2.1 依赖

```bash
go get github.com/dzm2020/gas
```

Go 1.25.3+；依赖见 `go.mod`（如 zap、redis、nats、consul、viper、protobuf 等）。

### 2.2 最小单节点示例（仅 Actor）

```go
import (
    "github.com/dzm2020/gas/internal/actor"
    "github.com/dzm2020/gas/internal/iface"
    "github.com/dzm2020/gas/pkg/lib/serializer"
)

// 实现 IActor
type MyActor struct { iface.Actor }
func (a *MyActor) OnInit(ctx iface.IContext, params []interface{}) error { return nil }
func (a *MyActor) OnMessage(ctx iface.IContext, msg interface{}) error   { return nil }
func (a *MyActor) OnStop(ctx iface.IContext) error                        { return nil }

sys := actor.NewSystem(nodeID, serializer.Json)
pid := sys.Spawn(&MyActor{}, arg1, arg2)
msg := iface.NewActorMessage(from, pid, "MethodName", data)
_ = sys.Send(msg)
// 或 data, err := sys.Call(msg)
defer sys.Shutdown()
```

### 2.3 使用 Node + Gate（推荐）

1. **配置文件**（如 `conf.yaml`）：  
   包含 `node`、`cluster`、`logger`、`gate` 等段，格式见各模块文档与 profile 的 Get/GetCluster/GetLogger。

2. **实现 Agent 的 IHandler**：

```go
type MyHandler struct { agent.Handler }
func (h *MyHandler) OnInit(agent gateiface.IAgent) error { return nil }
func (h *MyHandler) OnRoute(agent gateiface.IAgent, data []byte) error {
    // 使用 agent.GetSession().Response(data) 或 Push 回包
    return nil
}
func (h *MyHandler) OnStop(agent gateiface.IAgent) error { return nil }
```

3. **启动节点并挂载 Gate**：

```go
import (
    "github.com/dzm2020/gas/internal/node"
    "github.com/dzm2020/gas/internal/component/gate"
)

n := node.New("conf.yaml")
n.Startup(
    gate.NewComponent(), // 从 profile 读 "gate" 配置并启动
)
// Startup 会阻塞直到收到退出信号
```

4. **Gate 配置**（profile 中 `gate`）：  
   `address`（如 `tcp://0.0.0.0:9000`）、`max_conn`、可选 `options`；业务通过 `Factory` 返回的 IHandler 注入到 Gate。

### 2.4 集群模式

- 配置中关闭单节点模式（如 `single-node: false`），并配置 `cluster.discovery`（如 consul）、`cluster.messageQueue`（如 nats）。
- Node 启动时会创建 Cluster 组件并 Register 本节点；System 会使用 `actor.NewClusterSystem`，跨节点 Send/Call 经 cluster 的 MQ 与 Discovery 完成。
- 业务侧仍使用 `system.Send(msg)` / `system.Call(msg)`，目标 Pid 的 NodeId 非本节点时由 ClusterSystem 自动转发。

### 2.5 按需挂载组件

- **Redis**：`redis.NewComponent()` 加入 Startup，profile 中配置 `redis` 数组。
- **自定义组件**：实现 `component.IComponent[iface.INode]`，在 `Start(ctx, node)` 中从 `node` 取 System/Cluster 等，并注册到 Node 的 Manager；按注册顺序启动、逆序停止。

---

## 三、使用注意事项

1. **配置与 profile**  
   - 必须先调用 `profile.Init(path)`（Node.Startup 内部会调），否则 Get/GetCluster/GetLogger 可能未加载或 Fatal。  
   - 单节点/集群由配置决定；单节点下 Cluster 组件不启动，System 为单机版。

2. **Actor 与 Session**  
   - Actor 侧只依赖 `iface.ISession`、`iface.ISessionFactory`；Session 的“写回”能力由 gate 的 Session 与 ITransport 实现，集群下需保证 Push/Response 能路由到正确网关与连接。  
   - 有路由时 `ctx.Message()` 的语义为“当前正在处理的消息”；若需在异步回调中使用，注意生命周期与竞态。

3. **System.Shutdown**  
   - 仅向所有进程投递关闭任务，**不等待**进程完全退出（Mailbox 排空、Unregister 完成）。若需“全部停完再返回”，需在外部自建等待（如轮询 GetAllProcesses 或扩展 Wait 接口）。

4. **连接与 Agent**  
   - 每条连接一个 Agent（一个 Pid）；连接断开时 Gate 会 ShutdownProcess(pid)，Agent 在 OnStop 前会处理完 Mailbox 中消息。  
   - 超过 MaxConn 时 OnConnect 返回错误，连接会被关闭；计数在 OnClose 时减少。

5. **集群与序列化**  
   - 跨节点消息与 Session 会经过序列化；Session 内 Message 在 Values 中以 base64 存储，避免 JSON 对非法 UTF-8 的替换导致问题。  
   - Call 超时由 message.Deadline 或传入的 timeout 控制；超时后调用方收到错误，对端可能仍会处理完再丢弃回复。

6. **Router 与命名**  
   - 首字母大写的 Actor 方法可通过 Router 自动注册；集群下首字母大写的 Named 名字会同步到集群 Tags，便于 Select(tag, strategy) 寻址。  
   - globalRouterManager 为包级单例，多 System 共享同一 Router 缓存；若需测试隔离，可考虑依赖注入 Router。

7. **日志与 panic**  
   - 使用 pkg/glog 前需 Init；Node 会设置 panic 处理和 profile，建议通过 Node.Startup 启动以保证顺序。  
   - 在非 Node 场景单独使用 actor/network 时，需自行初始化 glog 与 profile（若用到）。

8. **依赖与 init**  
   - discovery 与 messageQue 的 provider（如 consul、nats）通过 `import _ "pkg/.../provider/xxx"` 注册；若未导入则 NewFromConfig 会报 unsupported type。

9. **协议与 codec**  
   - Gate 使用固定 13 字节头 + 变长 Body，单条最大 1MB；若需其他协议，可替换 Gate 的解码/编码或自行实现 IHandler 层之上的编解码。

10. **版本与兼容**  
    - 升级库时注意 internal/pb 与 proto 定义、iface 接口变更，以及 Cluster/Discovery/MQ 配置格式的兼容性。

---

## 四、模块索引

| 模块 | 文档 | 简要说明 |
|------|------|----------|
| internal/actor | [actor.md](modules/actor.md) | Actor 系统、进程、Mailbox、Router、Dispatcher |
| internal/iface | [iface.md](modules/iface.md) | 全局接口与类型（Pid、Message、Session、Node、System、Actor 等） |
| internal/node | [node.md](modules/node.md) | 节点生命周期与 Logger/Cluster/System 组件 |
| internal/profile | [profile.md](modules/profile.md) | 配置与 Profile 组件 |
| internal/component/gate | [gate.md](modules/gate.md) | 网关、Agent、Session、协议、codec、中间件 |
| internal/component/db/redis | [db-redis.md](modules/db-redis.md) | Redis 多实例组件 |
| internal/pb | [pb.md](modules/pb.md) | Protobuf 生成类型 |
| pkg/cluster | [cluster.md](modules/cluster.md) | 集群通信、ICluster、Send/Call、Select |
| pkg/discovery | [cluster.md](modules/cluster.md) | 服务发现，文档已合并至 cluster |
| pkg/messageQue | [cluster.md](modules/cluster.md) | 消息队列，文档已合并至 cluster |
| pkg/network | [network.md](modules/network.md) | TCP/UDP/WebSocket 与 IHandler/IConnection/IServer |
| pkg/glog | [glog.md](modules/glog.md) | 基于 zap 的全局日志 |
| pkg/lib | [lib.md](modules/lib.md) | 序列化、stopper、component、mpsc、xerror 等工具 |

更多细节、接口列表与协程/结构关系见各模块文档。
