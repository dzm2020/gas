# internal/component/gate 模块文档

## 1. 模块功能概述

`internal/component/gate` 实现网关组件：监听 TCP/UDP/WebSocket、管理连接、解包协议消息，并将客户端数据通过 Actor 提交给每连接一个的 Agent 处理；同时提供 Session/Transport 用于响应与集群下发。

- **Gate**：实现 `network.IHandler`，在 OnConnect 时为每条连接 Spawn 一个 Agent（绑定 Pid 到 connection context），OnMessage 解包后投递到对应 Agent 的 OnData，OnClose 时减计数并 ShutdownProcess(pid)。
- **Agent**：每个连接一个 Actor，持有 entity、Session、中间件链；OnData 经 AfterDecode 后交给 IHandler.OnRoute；Push 前经 BeforeEncode；支持远程回调 HandlerPush、HandlerSetValue、HandlerShutdown。
- **Session**：封装 *pb.Session，提供 Response/Push/Close/SyncValues 等写能力，通过 ITransport 下发到对端；Message 在 Values 中以 base64 存储以便集群序列化。
- **protocol**：固定 13 字节头（Len/Cmd/Act/Error/Index/Tag）+ 变长 Body。
- **codec**：大端序编解码，单条最大 1MB。
- **middleware**：AfterDecode / BeforeEncode 链，可选 log、compress、encrypt、ratelimit 等。
- **gateiface**：定义 IAgent、IMiddleware，供 agent 与 middleware 共用，避免循环依赖。

## 2. 接口文档

### 2.1 Gate

| 方法 | 说明 |
|------|------|
| `Start(ctx, system iface.ISystem) error` | 保存 system，设置 SessionFactory，创建 network server 并 Start |
| `OnConnect(entity IConnection) error` | 超过 MaxConn 返回错误；否则 count++，Spawn Agent 并 SetContext(pid) |
| `OnMessage(entity, data []byte) (n int, err error)` | 循环 codec.Decode，每条 process 投递到 entity 的 Agent OnData |
| `OnClose(entity, wrong error)` | count--，ShutdownProcess(pid) |
| `Stop(ctx) error` | server.Shutdown(ctx) |

配置字段：Address、Options、Factory（agent.Factory）、MaxConn；Factory 通过 `Factory()` 调用得到 IHandler。

### 2.2 gate/agent

| 类型 | 说明 |
|------|------|
| `Factory` | `func() IHandler`，每连接一个 Handler |
| `IHandler` | OnInit(agent)、OnRoute(agent, data)、OnStop(agent) |
| `Handler` | 空实现，可嵌入重写 |
| `IRemoteHandler` | HandlerPush、HandlerSetValue、HandlerShutdown（Actor 远程调用用） |

Agent 方法：OnInit/OnData/OnStop、GetEntity、Context、GetSession、AppendMiddleware/SetMiddleware/GetMiddleware、Push、SetValues、Shutdown；以及 IRemoteHandler 的三个实现。

### 2.3 gate/session

| 类型 | 说明 |
|------|------|
| `Factory` | 实现 iface.ISessionFactory，FromRaw(ctx, raw) -> New(raw, ctx) |
| `Session` | 嵌 *pb.Session，持 transport、msg；SetString/GetString、SetUint64/GetUint64、SetInt64/GetInt64、SyncValues、Response、ResponseErr、Push、Close、SetMessage、GetMessage、Raw |

### 2.4 gate/protocol

| 常量/类型 | 说明 |
|-----------|------|
| `HeadLen` | 13 |
| `Message` | *Head + Data []byte；New(cmd, act, data)、NewData(data)、NewErr(err)、Copy(old)、ID() |
| `Head` | Len、Cmd、Act、Error、Index、Tag 及 Get/Set |
| `CmdAct(cmd, act uint8) uint16` / `ParseId(msgId uint16) (cmd, act uint8)` | 组合/解析 |

### 2.5 gate/codec

| 函数/常量 | 说明 |
|-----------|------|
| `MaxMsgSize` | 1MB |
| `Encode(msg *protocol.Message) ([]byte, error)` | 大端序 13 字节头 + Body |
| `Decode(buf []byte) (*protocol.Message, int, error)` | 返回消息与消费字节数 |

### 2.6 gate/middleware

| 函数 | 说明 |
|------|------|
| `RunAfterDecode(chain, agent, msg)` | 顺序执行 AfterDecode，error 或 nil msg 即终止 |
| `RunBeforeEncode(chain, agent, msg)` | 顺序执行 BeforeEncode |

实现包内可含 log、compress、encrypt、ratelimit 等实现 gateiface.IMiddleware。

### 2.7 gateiface

| 接口 | 说明 |
|------|------|
| `IAgent` | Context、GetEntity、GetSession、SetMiddleware、AppendMiddleware、GetMiddleware、Push、SetValues、Shutdown |
| `IMiddleware` | AfterDecode(agent, msg)、BeforeEncode(agent, msg) |

### 2.8 Component（挂载到 Node）

| 方法 | 说明 |
|------|------|
| `Name() string` | "gate" |
| `Start(ctx, node) error` | profile.Get("gate", conf)，填充 Gate 配置后 Gate.Start(ctx, node.System()) |
| `Stop(ctx) error` | Gate.Stop(ctx) |

错误：`ErrNoAgent`（连接未绑定 Agent 时投递消息）。

## 3. 设计结构（协程模型与 struct 关系）

### 3.1 协程模型

- **network 层**：每个 listener 或连接可能有独立 goroutine（由 pkg/network 决定）；Gate 的 OnConnect/OnMessage/OnClose 在这些 goroutine 中调用。
- **OnMessage**：解码后通过 `system.SubmitTask(pid, task)` 将 OnData 投递到 Agent 的 Mailbox，在 Actor 的 Dispatcher goroutine 中执行；不阻塞网络读。
- **Session 写**：Response/Push 等经 transport（实现 ITransport）下发到对端；集群场景下 transport 可能通过 MQ 发到网关再写连接。

### 3.2 Struct 关系

```
Gate
  ├── Address, Options, Factory, MaxConn, count (atomic)
  ├── system iface.ISystem
  └── server network.IServer

Agent (每连接一个 Actor)
  ├── iface.Actor, IHandler
  ├── ctx IContext, session *session.Session, entity IConnection
  └── middlewares []gateiface.IMiddleware

Session
  ├── *pb.Session
  ├── transport ITransport (ctx + agent Pid，用于 Push/Close/SetValue)
  └── msg *protocol.Message (当前请求，可 base64 存 Values[KeyMessage])

Message (protocol)
  ├── *Head (Len,Cmd,Act,Error,Index,Tag)
  └── Data []byte
```

- **Gate → Agent**：OnConnect 时 `pid := system.Spawn(agent.New(entity, Factory()))`，entity.SetContext(pid)。
- **Agent → Session**：OnInit 时 session = session.New(&pb.Session{...}, ctx)；Session 的 transport 指向能向该 Agent 发 Push/SetValue/Close 的实现（如本机直接写 entity，集群经 MQ 到网关）。
- **gateiface**：仅接口，被 agent 与 middleware 引用，避免 agent → middleware → agent 循环。

### 3.3 依赖

- `internal/iface`、`internal/pb`、`internal/component/gate/agent`、`session`、`codec`、`protocol`、`middleware`、`gateiface`
- `pkg/network`、`pkg/glog`、`pkg/lib/xerror`、`internal/profile`、`pkg/lib/component`
