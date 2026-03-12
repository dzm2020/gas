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
factory := func() gateiface.IAgentHandler { return &MyHandler{} }
n.Startup(
    gate.NewComponent(factory), // 从 profile 读 "gate" 配置并启动
)
// Startup 会阻塞直到收到退出信号
```

4. **Gate 配置**（profile 中 `gate`）：  
   `address`（如 `tcp://0.0.0.0:9000`）、`max_conn`、可选 `options`；业务通过 `Factory` 返回的 IHandler 注入到 Gate。

### 2.4 集群模式示例

配置中关闭单节点模式并配置服务发现与消息队列后，Node 会创建 Cluster 组件并注册本节点，System 使用 `ClusterSystem`，跨节点 Send/Call 经 cluster 自动完成。**集群由多个进程组成**：同一份程序，用**不同配置文件**（不同 node.id、端口、监听地址）启动多个进程，各进程向同一套 discovery/mq 注册，即形成集群。

**节点 1 配置**（如 `conf/node1.yaml`）：

```yaml
single-node: false

node:
  kind: game
  id: 1
  address: "127.0.0.1"
  port: 9001
  tags: ["gate"]

cluster:
  name: my-cluster
  discovery:
    type: consul
    config: { ... }
  messageQueue:
    type: nats
    config: { ... }

logger: { ... }
gate:
  address: "tcp://0.0.0.0:9001"
  max_conn: 10000
```

**节点 2 配置**（如 `conf/node2.yaml`）：仅 node.id、port、gate.address 等与节点 1 区分，cluster/logger 等一致。

```yaml
single-node: false

node:
  kind: game
  id: 2
  address: "127.0.0.1"
  port: 9002
  tags: ["gate"]

cluster: { ... }   # 与 node1 相同
logger: { ... }
gate:
  address: "tcp://0.0.0.0:9002"
  max_conn: 10000
```

**同一份 main：通过传入的配置路径区分节点**：

```go
package main

import (
	"os"
	"github.com/dzm2020/gas/internal/node"
	"github.com/dzm2020/gas/internal/component/gate"
	"github.com/dzm2020/gas/internal/component/gate/gateiface"
)

func main() {
	configPath := "conf/node1.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	n := node.New(configPath)
	factory := func() gateiface.IAgentHandler {
		return &MyHandler{}
	}

	_ = n.Startup(gate.NewComponent(factory))
}
```

**启动多个节点**（多进程、同机或不同机）：

```bash
# 终端 1：节点 1
./myapp conf/node1.yaml

# 终端 2：节点 2
./myapp conf/node2.yaml

# 更多节点同理，或通过环境变量、flag 等传入配置路径
```

两进程启动后，会分别向 discovery 注册、订阅 MQ；业务侧使用 `system.Send(msg)` / `system.Call(msg)` 时，目标 Pid 的 NodeId 非本节点则由 ClusterSystem 自动转发到对应节点。

**节点间相互通信**：在能拿到 `node` 的地方（如自定义组件、或在创建 Handler 时注入 node），用 `node.Cluster().Select(tag, strategy)` 选目标节点，再构造目标节点上的 `Pid`，通过 `ctx.Send` / `ctx.Call` 发送即可；若目标在本节点则走本地，在其它节点则自动走集群转发。

示例：本节点某 Actor 向带 `"gate"` 标签的其它节点上的 Named Actor `"Worker"` 发异步消息或同步请求。

```go
import (
	"time"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/cluster"
)

// 假设当前在某个 Actor 的方法里，ctx 为 IContext，且能拿到 node（如通过 struct 注入）
func callOtherNode(ctx iface.IContext, node iface.INode, data []byte) error {
	clu := node.Cluster()
	if clu == nil {
		return nil // 单节点模式
	}

	// 按 tag 选一个节点（如 "gate"），策略可选 RouteRandom、RouteRoundRobin 等
	targetNodeId, err := clu.Select("gate", cluster.RouteRandom)
	if err != nil {
		return err
	}

	// 构造目标节点上的进程：按名字寻址（对方需 Named("Worker")）
	toPid := iface.NewPidWithName("Worker", targetNodeId)

	// 异步发送：不等待回复
	if err := ctx.Send(toPid, "DoWork", data); err != nil {
		return err
	}
	return nil
}

// 同步调用：等待回复或超时
func callOtherNodeSync(ctx iface.IContext, node iface.INode, req, reply interface{}) error {
	clu := node.Cluster()
	if clu == nil {
		return nil
	}
	targetNodeId, err := clu.Select("gate", cluster.RouteRandom)
	if err != nil {
		return err
	}
	toPid := iface.NewPidWithName("Worker", targetNodeId)
	ctx.SetCallTimeout(5 * time.Second)
	return ctx.Call(toPid, "DoWork", req, reply)
}
```

- **Select**：`cluster.Select(tag, strategy)` 从 discovery 按 tag 取成员列表，用策略选一个，返回其 `nodeId`。
- **Pid**：`iface.NewPid(nodeId, actorId)` 按进程 ID 寻址；`iface.NewPidWithName("Worker", nodeId)` 按 Named 名字寻址（目标节点上需已 `Named("Worker")`）。
- **Send/Call**：与单节点用法一致；目标 Pid 的 NodeId 非本节点时由 ClusterSystem 经 MQ 转发到对应节点并投递到目标 Actor。

### 2.5 组件注册示例

Node 内置 Profile、Logger、Cluster、System 四个组件；`Startup(comps ...)` 中传入的组件会按**注册顺序**启动、**逆序**停止。

**挂载 Gate**（需提供 AgentHandler 工厂）：

```go
n := node.New("conf.yaml")
factory := func() gateiface.IAgentHandler { return &MyHandler{} }
_ = n.Startup(gate.NewComponent(factory))
```

**挂载 Redis**（profile 中配置 `redis` 数组）：

```go
import "github.com/dzm2020/gas/internal/component/db/redis"

n := node.New("conf.yaml")
_ = n.Startup(
	gate.NewComponent(factory),
	redis.NewComponent(),
)
```

**自定义组件**：实现 `IComponent[iface.INode]`，在 `Start(ctx, node)` 中从 `node` 取 System、Cluster、Profile 等使用：

```go
import (
	"context"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/lib/component"
)

const MyCompName = "mycomp"

type MyComponent struct {
	component.BaseComponent[iface.INode]
}

func NewMyComponent() *MyComponent {
	return &MyComponent{}
}

func (c *MyComponent) Name() string { return MyCompName }

func (c *MyComponent) Start(ctx context.Context, node iface.INode) error {
	// 使用 node.System()、node.Cluster()、node.Profile() 等
	_ = node.System()
	_ = node.Cluster()
	return nil
}

// 使用
n := node.New("conf.yaml")
_ = n.Startup(
	gate.NewComponent(factory),
	NewMyComponent(),
)
```

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

以下为各模块文档入口，接口与实现细节见对应链接。

| 模块 | 文档 | 说明 |
|------|------|------|
| internal/actor | [actor.md](modules/actor.md) | Actor 系统、进程、Mailbox、Router、路由函数 |
| internal/iface | [iface.md](modules/iface.md) | Pid、Message、Session、Node、System、Actor 等接口 |
| internal/node | [node.md](modules/node.md) | 节点生命周期与内置组件 |
| internal/profile | [profile.md](modules/profile.md) | 配置与 Profile 组件 |
| internal/component/gate | [gate.md](modules/gate.md) | 网关、Agent、Session、协议、codec、中间件 |
| internal/component/db/redis | [db-redis.md](../tools/discard/db-redis.md) | Redis 多实例组件 |
| internal/pb | [pb.md](modules/pb.md) | Protobuf 生成类型 |
| pkg/cluster | [cluster.md](modules/cluster.md) | ICluster、Send/Call、Select（discovery/messageQue 见该文档） |
| pkg/network | [network.md](modules/network.md) | TCP/UDP/WebSocket、IHandler/IConnection/IServer |
| pkg/glog | [glog.md](modules/glog.md) | 基于 zap 的全局日志 |
| pkg/lib | [lib.md](lib.md) | 序列化、stopper、component、mpsc、xerror 等 |

更多细节、接口列表与设计结构见各模块文档。
