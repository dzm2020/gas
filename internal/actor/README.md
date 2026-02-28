# internal/actor — Actor 模型实现

本包提供基于 Actor 模型的并发抽象：单节点/集群系统、进程与名字管理、消息与任务派发、路由与调度器、优雅关闭。

---

## 一、代码质量评估

### 1. 总体评价

| 维度         | 评分 (1–5) | 说明 |
|--------------|------------|------|
| 架构清晰度   | 5          | 职责划分明确：System/ClusterSystem、Process、Context、Router、Mailbox、Dispatcher 边界清楚 |
| 接口设计     | 5          | 依赖 `internal/iface` 抽象，便于测试与扩展；`var _ iface.XX = (*impl)(nil)` 编译期校验 |
| 并发安全     | 5          | 使用 `atomic`、`ConcurrentMap`、`sync.RWMutex`、MPSC 队列，无裸 map 并发写 |
| 可测试性     | 4          | 单测覆盖 System 核心路径；Router/Mailbox/ClusterSystem 可再增加针对性测试 |
| 可维护性     | 4          | 注释与包级说明到位；少数分支逻辑可再拆函数以降低圈复杂度 |
| 错误处理     | 4          | 包级 sentinel 错误、`xerror.Wrap` 包装；个别处可统一错误码或类型 |
| 文档与命名   | 4          | 文件头注释、关键函数有说明；部分内部类型可补充 godoc |

**综合**：生产可用，结构清晰、并发安全、测试通过；在测试覆盖、边界情况与文档细节上仍有小幅提升空间。

### 2. 优点

- **接口驱动**：`ISystem`、`IContext`、`IActor`、`IProcess`、`IRouter` 等定义在 `internal/iface`，实现可替换、易 mock。
- **单节点与集群分离**：`System` 负责本节点，`ClusterSystem` 嵌入 `*System` 并委托 `cluster.ICluster` 做跨节点 Send/Call 与全局命名，扩展清晰。
- **进程生命周期明确**：Spawn → Register → (Named) → 消息/任务 → Shutdown → Unregister/OnStop，关闭通过 Stopper + 退出任务保证顺序。
- **路由与调度解耦**：Router 基于反射做方法路由；Dispatcher 支持 goroutine / 同步两种调度；Mailbox 用 CAS 保证单 goroutine 处理，throughput 控制公平性。
- **单测覆盖核心路径**：`actor_test.go` 覆盖 System 构造、注册/注销、Named/Unname、Spawn、SubmitTaskAndWait、Shutdown、Send 关闭后行为、进程不存在等。

### 3. 待改进点

- **context.go — 有路由时未清空 `a.msg`**  
  有路由时在 `handleMessage` 中只做 `a.msg = m` 与 `Response`，未在返回前执行 `a.msg = nil`。与“无路由走 OnMessage”路径不一致，若下游依赖 `Message()` 的“仅当前消息有效”语义，可能读到旧消息。建议：有路由分支末尾也执行 `a.msg = nil`（或统一在函数末尾清空）。

- **context.go — 无路由时的日志**  
  “actor没有找到消息路由,执行默认方法” 在已执行 `OnMessage` 之后打印，易被理解为“没做任何处理”。可改为“未匹配路由，已通过 OnMessage 处理”或仅在需要排查时打 debug。

- **System.Shutdown 不等待进程完全退出**  
  当前只向所有进程发 Shutdown，不等待 mailbox 排空或进程从 IdDict 移除。若调用方需要“全部停完再返回”，需在外部自建等待（例如轮询 GetAllProcesses 或增加 Wait 接口）。

- **测试覆盖**  
  Router 的各类 handler 签名、ClusterSystem 的 Named/Unname 与跨节点 Send/Call、Mailbox 的 schedule/run、Dispatch 的 panic 恢复等，可增加单元测试或集成测试。

- **router_mgr 全局单例**  
  `globalRouterManager` 为包级变量，多 System/多节点共享同一 router 缓存。当前按 actor 类型缓存无状态 Router 一般没问题，若未来做测试隔离或多租户，可考虑通过依赖注入传入 Router 或 Manager。

---

## 二、架构概览

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

- **System**：单节点。维护 `IdDict`、`nameDict`，提供 Spawn、Register、Named、Send、Call、SubmitTask、Shutdown 等。
- **ClusterSystem**：嵌入 `*System`，重写 Send/Call 判断本节点/跨节点；Named/Unname 对首字母大写的名字同步到集群 Tags。
- **Process**：对 Mailbox + Stopper 的封装；PostMessage、Shutdown（投递退出任务）。
- **actorContext**：实现 IContext，持有 process/pid/actor/router/node/system；消息由 Router 或 Actor.OnMessage 处理；提供 Send/Call/Forward、AfterFunc、Named/Unname 等。
- **Router**：按方法名反射注册；支持同步/异步/会话等多种方法签名，反序列化后调用。
- **router_mgr**：按 actor 类型缓存 Router，GetRouterForActor/RegisterRouter。
- **Mailbox**：无界 MPSC 队列 + CAS 调度，保证同一时刻只有一个 goroutine 执行 run，throughput 控制 Gosched。
- **dispatch.go**：默认 goroutine 派发与同步派发，带 panic recover。

---

## 三、文件职责

| 文件              | 职责 |
|-------------------|------|
| `system.go`       | 单节点 System：进程表/名字表、Spawn/Register/Unregister、Send/Call/SubmitTask、Shutdown |
| `cluster_system.go` | 集群 System：本地走 System，跨节点走 transport；全局名 Named/Unname |
| `context.go`      | actorContext：IContext 实现、消息处理与路由、Send/Call/Forward、定时器、Named/Unname |
| `process.go`      | Process：IProcess 实现、PostMessage、Shutdown（投递退出任务） |
| `mailbox.go`      | Mailbox：队列、RegisterHandlers、PostMessage、CAS schedule + run 循环 |
| `dispatch.go`     | IDispatcher：goroutine / synchronized 两种调度，Throughput、panic 恢复 |
| `router.go`       | Router：反射扫描注册、多种 handler 签名、Handle/HasRoute、同步/异步/会话消息 |
| `router_mgr.go`   | 全局 Router 缓存，GetRouterForActor / RegisterRouter |
| `actor_test.go`   | System 相关单测 |

---

## 四、使用要点

- **创建系统**：单节点 `NewSystem(node)`，集群 `NewClusterSystem(node, transport)`。
- **创建 Actor**：`pid := system.Spawn(actor, args...)`；实现 `IActor`（OnInit/OnMessage/OnStop），可选通过导出方法自动注册路由。
- **发消息**：`system.Send(msg)` / `system.Call(msg)`；Context 内 `ctx.Send(pid, method, req)` / `ctx.Call(pid, method, req, reply)`。
- **任务**：`system.SubmitTask(pid, task)` 异步，`SubmitTaskAndWait(pid, task, timeout)` 同步等待。
- **命名**：`ctx.Named(name)` / `ctx.Unname()`；集群下首字母大写的名字会同步到集群 Tags。
- **关闭**：`system.ShutdownProcess(pid)` 关闭单进程；`system.Shutdown()` 关闭整个系统（不等待进程完全退出，需时可在外部自建等待）。

---

## 五、依赖

- `internal/iface`：ISystem、IContext、IActor、IProcess、IRouter、IMessage 等。
- `internal/gate/session`：会话类型路由。
- `pkg/glog`、`pkg/lib`、`pkg/lib/stopper`、`pkg/lib/xerror`、`pkg/cluster`、`pkg/discovery/iface`、`github.com/duke-git/lancet/v2`、`go.uber.org/zap` 等。

---

*文档与评估基于当前代码快照，后续实现变更请同步更新本文档。*
