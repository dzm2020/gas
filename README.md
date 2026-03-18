# GAS

**GAS**（Gateway-Actor-System）是一个用 Go 编写的分布式长连接与游戏服务框架。它围绕 **Actor 模型** 组织业务逻辑，通过 **Node** 统一节点生命周期，提供 **Gate** 网关处理客户端连接，用 **Cluster** 做服务发现与跨节点通信，适合需要高并发、有状态会话、多节点协作的场景。

## 特性

- **Actor 模型**：状态与逻辑封装在 Actor 内，单 Actor 内消息串行，避免共享可变状态；支持 `Spawn`、`Send`、`Call`、命名注册与任务投递。
- **节点与组件**：`Node` 管理配置加载（Profile）、日志（Logger）、集群（Cluster）、Actor 系统（System）及可选 Gate、Redis 等组件的启停与依赖顺序。
- **Gate 网关**：监听 TCP / UDP / WebSocket，为每条连接绑定一个 Agent（Actor），协议解包后消息投递到对应 Agent，通过 Session 做响应与推送；支持中间件（日志、压缩、加密、限流等）。
- **集群**：基于服务发现（如 Consul）与消息队列（如 NATS）实现节点注册、按节点 Send/Call、按 Tag 选节点、拓扑变更监听。
- **可扩展**：核心能力通过接口抽象（`internal/iface`），序列化支持 JSON / MsgPack / Protobuf，组件可按需挂载。

## 安装

```bash
go get github.com/dzm2020/gas
```

- **Go**：1.25.3+（见 `go.mod`）
- **主要依赖**：zap、viper、redis、nats、consul、protobuf、gorilla/websocket 等。

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
