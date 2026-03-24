# GAS

**GAS**（Gateway-Actor-System）是一个用 Go 编写的分布式长连接与游戏服务框架。它围绕 **Actor 模型** 组织业务逻辑，通过 **Node** 统一节点生命周期，提供 **Gate** 网关处理客户端连接，用 **Cluster** 做服务发现与跨节点通信，适合需要高并发、有状态会话、多节点协作的场景。

## 特性

- **Actor 模型**：状态与逻辑封装在 Actor 内，单 Actor 内消息串行，避免共享可变状态；支持 `Spawn`、`Send`、`Call`、命名注册与任务投递。
- **节点与组件**：`Node` 管理配置加载（Profile）、日志（Logger）、集群（Cluster）、Actor 系统（System）及可选 Gate、Redis 等组件的启停与依赖顺序。
- **Gate 网关**：监听 TCP / UDP / WebSocket，为每条连接绑定一个 Agent（Actor），协议解包后消息投递到对应 Agent，通过 Session 做响应与推送；支持中间件（日志、压缩、加密、限流等）。
- **集群**：基于服务发现（如 Consul）与消息队列（如 NATS）实现节点注册、按节点 Send/Call、按 Tag 选节点、拓扑变更监听。
- **事件总线**：基于 **topic** 字符串与 **`[]byte` 载荷**的发布订阅；支持本节点 **`PublishLocal`** 与集群 **`PublishCluster`**（经 MQ 广播后各节点再本地分发）；订阅回调通过 **`SubmitTask`** 在**订阅者 Actor 的邮箱协程**中与 `OnMessage` 同模型串行执行。
- **可扩展**：核心能力通过接口抽象（`internal/iface`），序列化支持 JSON / MsgPack / Protobuf，组件可按需挂载。

## 安装

```bash
go get github.com/dzm2020/gas
```

- **Go**：1.25.3+（见 `go.mod`）；CI 与本地请对齐同一工具链版本，避免行为差异。
- **主要依赖**：zap、viper、[go-redis v9](https://github.com/redis/go-redis)、nats、consul、protobuf、gorilla/websocket 等。

### 模块边界说明

示例与文档中的 `import "github.com/dzm2020/gas/internal/..."` **仅在本仓库根模块内有效**。其他项目执行 `go get` 后**不能直接** import 本仓库的 `internal` 路径；可复用能力主要在 `pkg/`。详见 [`docs/lib.md`](docs/lib.md) 开头「模块边界」一节。

## 快速开始

### 单节点 + Gate（常见用法）

1. 准备配置文件（如 `config.yml`），包含 `node`、`logger`、`gate` 等段；单节点可设 `single-node: true` 以跳过集群。

2. 实现网关业务处理（每个连接对应一个 Agent）：

```go
package main

import (
	"github.com/dzm2020/gas/internal/component/gate"
	"github.com/dzm2020/gas/internal/component/gate/gateiface"
	"github.com/dzm2020/gas/internal/node"
)

type MyHandler struct{ gateiface.BaseAgentHandlerFactory }

func (h *MyHandler) OnInit(agent gateiface.IAgent) error   { return nil }
func (h *MyHandler) OnRoute(agent gateiface.IAgent, data []byte) error { return nil }
func (h *MyHandler) OnStop(agent gateiface.IAgent) error   { return nil }

func main() {
	n := node.New()
	n.SetConfigPath("config.yml")
	n.SetConfigType("yaml")
	_ = n.Startup(gate.NewComponent(&MyHandler{}))
}
```

3. 在配置中填写 Gate 的 `address`（如 `tcp://0.0.0.0:9000`）、`maxConn` 等；`Startup` 会阻塞直至收到退出信号并完成优雅关闭。

### 仅用 Actor 系统（无 Gate）

若不使用 Gate，只需在业务里挂载自己的组件，或直接使用内置的 Profile / Logger / Cluster / System。Node 默认会按顺序启动 Profile、Logger、Cluster、System；`n.System()` 取得 `ISystem` 后即可 `Spawn` Actor、`Send`/`Call` 消息。

```go
sys := n.System()
pid := sys.Spawn(&MyActor{}, arg1, arg2)
msg := iface.NewActorMessage(from, pid, "MethodName", data)
_ = sys.SendMessage(msg)
```

## 配置与组件

- **Profile**：从 YAML/JSON 等加载配置，首项启动，并填充 `node.Member`（如 kind、id、address、port、tags）。
- **Logger**：基于 zap，可由 Profile 的 `logger` 段配置。
- **Cluster**：在非单节点模式下，根据 `cluster` 段创建服务发现与消息队列，启动后向集群注册本节点；跨节点消息经 Cluster 转发。
- **System**：本节点 Actor 系统，负责进程表、名字表、本地/跨节点 Send/Call。
- **Gate**：可选组件，从 Profile 的 `gate` 段读监听地址、最大连接数等并启动网络服务。

集群配置示例（`cluster` 段）：需配置 `discovery`（如 type: consul）和 `messageQueue`（如 type: nats），并设 `single-node: false`。同一程序多进程、不同配置文件（不同 node.id/端口）即可组成集群。

## 事件说明

框架内「事件」指 **Actor 事件总线**（`IContext` / `ISystem` 上的 `Subscribe`、`PublishLocal`、`PublishCluster`），与 **`Send`/`Call` 的 Actor 消息**是两套机制：事件按 **topic** 多播给所有订阅者，载荷为 **`[]byte`**，由业务自行约定编码（如 JSON、Protobuf）。

| 能力 | 说明 |
|------|------|
| **`Subscribe`** | 以**当前 Actor**为订阅者注册 `EventHandler`；仅允许订阅**本节点** Actor（`Pid` 与当前节点一致）。 |
| **`PublishLocal`** | 仅在本节点总线查找订阅表并投递，**不经消息队列**。 |
| **`PublishCluster`** | 将事件发到 MQ（subject 前缀 `gas.event.`，与节点收件箱用的数字 subject 隔离）；各节点收到后在本地再执行 **`PublishLocal`**。 |
| **纯本地 `NewSystem`** | 未挂载集群传输时，`PublishCluster` 会返回 **`ErrEventNoCluster`**；`PublishLocal` 始终可用。 |

**注意**：`PublishLocal` 会对**每个订阅者**复制一份 `payload` 再入队，高 QPS 或大 body 时注意开销；宜传小消息或传引用类标识由订阅方再拉数据。

**易混淆**：`pkg/lib/event` 的 **`Listener[V]`** 是进程内同步回调列表，**不是** Actor 事件总线；二者对比见 **[`docs/event.md`](docs/event.md)**（含流程说明与示例测试路径）。

## 项目结构概要

- `internal/node`：节点入口与 `Startup` 流程。
- `internal/iface`：Node、System、Actor、Cluster、Profile、Session 等接口定义。
- `internal/actor`：Actor 系统实现（进程、邮箱、派发、路由、跨节点桥接）。
- `internal/component`：Profile、Logger、Cluster、System、Gate、Redis 等节点组件。
- `pkg/cluster`：集群封装（发现 + 消息队列、Send/Call、按 Tag 选节点）。
- `pkg/discovery`：服务发现抽象与 Consul 实现。
- `pkg/messageQue`：消息队列抽象与 NATS 实现。
- `pkg/network`：TCP/UDP/WebSocket 服务与连接。
- `pkg/glog`、`pkg/lib/serializer`、`pkg/lib/component` 等：日志、序列化、组件管理。

更多细节可参考仓库内 `docs/` 文档。

## 文档索引

| 文档 | 说明 |
|------|------|
| [`docs/lib.md`](docs/lib.md) | `pkg/lib` 子包说明与模块边界 |
| [`docs/event.md`](docs/event.md) | Actor 事件总线；与 `pkg/lib/event` 的区别 |
| [`docs/config.md`](docs/config.md) | 配置文件结构与安全注意事项 |
| [`docs/message_flow.md`](docs/message_flow.md) | 消息流转 |
| [`docs/api.md`](docs/api.md) | API 相关说明 |
