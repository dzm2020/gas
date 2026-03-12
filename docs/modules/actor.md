# internal/actor 模块文档

## 1. 模块功能概述

`internal/actor` 实现基于 Actor 模型的并发抽象，提供：

- **单节点 / 集群系统**：`System` 负责本节点进程与消息；`ClusterSystem` 在之上增加跨节点 Send/Call 与全局命名。
- **进程与名字管理**：通过 `IdDict`（ActorId → IContext）、`nameDict`（名字 → IContext）管理进程；支持 Spawn、Register、Named、Unname。
- **消息与任务派发**：支持异步 Send、同步 Call、SubmitTask / SubmitTaskAndWait；消息经 Mailbox 入队后由 Dispatcher 调度执行。
- **路由与调度**：Router 基于反射按方法名路由；Dispatcher 支持 goroutine 与同步两种模式；Mailbox 使用 CAS 保证单 goroutine 处理，throughput 控制公平性。
- **优雅关闭**：Stopper 标记关闭状态；Shutdown 向所有进程投递退出任务，进程在处理完 Mailbox 后执行 OnStop 并 Unregister。

todo介绍下路由回调函数格式

## 2. 接口文档

### 2.1 对外构造与入口（本包提供，接口在 iface）

| 函数/方法 | 说明 |
|-----------|------|
| `NewSystem(selfNodeID uint64, serializer lib.ISerializer) *System` | 创建单节点 Actor 系统 |
| `NewClusterSystem(selfNodeID uint64, serializer lib.ISerializer, transport cluster.ICluster) *ClusterSystem` | 创建集群版 Actor 系统 |
| `GetRouterForActor(actor iface.IActor) iface.IRouter` | 按 actor 类型获取（或创建并缓存）Router |
| `NewProcess(mailbox IMailbox) *Process` | 创建进程（内部使用） |
| `NewMailbox() *Mailbox` | 创建邮箱（内部使用） |
| `NewDefaultDispatcher(throughput int) IDispatcher` | 创建 goroutine 调度器 |
| `NewSynchronizedDispatcher(throughput int) IDispatcher` | 创建同步调度器 |
| `NewRouter() iface.IRouter` | 创建路由器 |

### 2.2 System（实现 iface.ISystem）

| 方法 | 说明 |
|------|------|
| `NodeId() uint64` | 本节点 ID |
| `NextID() uint64` | 生成下一个 ActorId |
| `Serializer() lib.ISerializer` | 序列化器 |
| `SessionFactory() iface.ISessionFactory` / `SetSessionFactory(f)` | Session 工厂（可选，gate 注入） |
| `Spawn(actor iface.IActor, args ...interface{}) *Pid` | 创建并注册新进程，投递 OnInit 后返回 Pid |
| `Register(ctx IContext) error` | 将 Context 注册到 IdDict |
| `Unregister(ctx IContext) error` | 从 IdDict 删除并 Unname |
| `Named(ctx IContext) error` / `Unname(ctx IContext) error` | 注册/注销进程名字 |
| `SubmitTask(pid *Pid, task Task) error` | 异步投递任务 |
| `SubmitTaskAndWait(pid *Pid, task Task, timeout time.Duration) error` | 投递任务并等待完成 |
| `Send(message *ActorMessage) error` | 异步发送消息（仅本节点） |
| `Call(message *ActorMessage) (data []byte, err error)` | 同步发送并等待响应 |
| `GetProcess(ref interface{}) IProcess` | ref 可为 string \| uint64 \| *Pid |
| `GetAllProcesses() []IProcess` | 所有已注册进程 |
| `ShutdownProcess(pid *Pid) error` | 关闭指定进程 |
| `Shutdown() error` | 关闭系统（不等待进程完全退出） |

### 2.3 ClusterSystem

在 System 基础上重写：

| 方法 | 说明 |
|------|------|
| `Send(message *ActorMessage) error` | 本节点走 System.Send，跨节点走 transport.Send |
| `Call(message *ActorMessage) (data []byte, err error)` | 本节点走 System.Call，跨节点走 transport.Call 并反序列化 Response |

### 2.4 内部接口（actor 包内）

| 接口 | 说明 |
|------|------|
| `IMailbox` | `PostMessage`、`RegisterHandlers(invoker, dispatcher)`、`IsEmpty` |
| `IDispatcher` | `Schedule(f, recoverFun)`、`Throughput()` |

### 2.5 错误

- `ErrProcessExiting`、`ErrProcessNotFound`、`ErrMessageIsNil`、`ErrSystemShuttingDown`、`ErrNameAlreadyRegistered`
- Router：`ErrMessageHandlerNotFound`、`ErrHandlerReturnType`、`ErrUnknownHandlerType` 等

## 3. 设计结构（协程模型与 struct 关系）

### 3.1 协程模型

- **每进程一个 Mailbox**：无界 MPSC 队列；`PostMessage` 将消息 Push 后调用 `schedule()`。
- **schedule**：CAS 将状态从 `idle` 置为 `running`，仅一个 goroutine 进入；失败则直接返回（由已运行的 goroutine 继续消费队列）。
- **Dispatcher**：
  - **goroutine**：`Schedule` 时 `go fn()`，消息在独立 goroutine 中处理，带 panic recover。
  - **synchronized**：`Schedule` 时直接执行 `fn()`，在同一 goroutine 内同步处理。
- **run 循环**：从队列 Pop，调用 `invoker.InvokerMessage(msg)`；每处理 `Throughput` 条后 `runtime.Gosched()` 让出，避免饥饿。
- **Call 语义**：调用方 goroutine 通过 `ChanWaiter` 阻塞等待；目标进程在 Mailbox 的调度 goroutine 中执行并调用 `message.Response(data, err)` 唤醒。

### 3.2 Struct 关系图

```
                    +------------------+
                    |   ISystem        |
                    | (System /        |
                    |  ClusterSystem)  |
                    +--------+---------+
                             |
         +-------------------+-------------------+
         |                   |                   |
         v                   v                   v
  +-------------+    +---------------+   +----------------+
  | IdDict      |    | nameDict      |   | Spawn/Register |
  | (id->ctx)   |    | (name->ctx)   |   | Send/Call/Task |
  +-------------+    +---------------+   +----------------+
         |
         v
  +-------------+    +------------------+
  | IContext    |<---| actorContext     |
  | (actorContext)   | process, pid,    |
  +-------------+    | actor, router    |
         |            +------------------+
         v
  +-------------+    +------------------+
  | IProcess    |<---| Process          |
  | (Process)   |    | mailbox, Stopper |
  +------+------+    +------------------+
         |
         v
  +-------------+    +------------------+
  | IMailbox    |<---| Mailbox          |
  | (Mailbox)   |    | queue (Mpsc),    |
  +------+------+    | invoker, dispatch|
         |            +------------------+
         v
  +-------------+    +------------------+
  | IDispatcher |    | goroutine /      |
  |             |    | synchronized     |
  +-------------+    +------------------+
```

- **System**：持有 `IdDict`、`nameDict`、`sessionFactory`、`serializer`；ClusterSystem 嵌入 `*System` 并持有 `transport cluster.ICluster`。
- **actorContext**：实现 IContext，持有 process、pid、actor、router、system、sessionFactory；消息由 Router.Handle 或 Actor.OnMessage 处理。
- **Process**：封装 Mailbox + Stopper；Shutdown 时投递退出任务（Unregister + OnStop）到 Mailbox。
- **Router**：由 `router_mgr` 按 actor 类型缓存；支持同步/异步/会话等多种方法签名。

### 3.3 依赖

- `internal/iface`（ISystem、IContext、IActor、IProcess、IRouter、IMessage、ISessionFactory 等）
- `internal/pb`（Pid、Message 等）
- `pkg/glog`、`pkg/lib`（Mpsc、Timer、ChanWaiter、DeadlineToTimeout）、`pkg/lib/stopper`、`pkg/lib/xerror`
- `pkg/cluster`（ClusterSystem 跨节点）
- `github.com/duke-git/lancet/v2`（ConcurrentMap、convertor）
