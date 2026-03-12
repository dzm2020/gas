# 消息流程图：Client → Gate → Agent → 跨集群 Actor

本文档描述 GAS 框架中从客户端到网关、Agent，再到本节点或跨集群 Actor 的消息流向，以及反向回包路径。

---

## 1. 总体流程概览

```mermaid
flowchart LR
    subgraph Client["客户端"]
        C[Client]
    end
    subgraph NodeA["节点 A (Gate 节点)"]
        G[Gate]
        A[Agent]
        BA[业务 Actor]
    end
    subgraph Cluster["集群传输"]
        MQ[MessageQueue<br/>e.g. NATS]
    end
    subgraph NodeB["节点 B"]
        CS[Cluster 订阅]
        RA[跨集群 Actor]
    end

    C -->|"① TCP/二进制"| G
    G -->|"② SubmitTask → Agent"| A
    A -->|"③ OnRoute → Send/Call"| BA
    A -.->|"④ 跨节点 Send/Call"| MQ
    MQ -->|"⑤ Publish/Request"| CS
    CS -->|"⑥ System.Send/Call"| RA
    RA -.->|"⑦ Session.Response/Push"| A
    A -->|"⑧ Push → 连接"| C
```

---

## 2. 上行：Client → Gate → Agent → Actor（详细）

```mermaid
sequenceDiagram
    participant Client as 客户端
    participant Network as 网络层(TCP/UDP/WS)
    participant Gate as Gate
    participant Codec as Codec
    participant System as Actor System
    participant Agent as Agent
    participant Handler as IHandler.OnRoute
    participant LocalActor as 本节点 Actor
    participant Cluster as ClusterSystem
    participant MQ as MessageQueue
    participant RemoteNode as 目标节点 Cluster
    participant RemoteActor as 跨集群 Actor

    Client->>Network: 发送二进制数据
    Network->>Gate: OnMessage(conn, data)
    Gate->>Codec: Decode(data) → *protocol.Message
    Gate->>Gate: process(): 从 conn 取绑定的 Agent Pid
    Gate->>System: SubmitTask(pid, func: Agent.OnData(msg))
    System->>Agent: 投递到 Agent 的 Mailbox → OnData(msg)

    Agent->>Agent: middleware.RunAfterDecode
    Agent->>Agent: session.SetMessage(msg)
    Agent->>Handler: IHandler.OnRoute(agent, msg.Data)

    alt 转发到本节点 Actor
        Handler->>System: ctx.Send(localPid, "Method", data)
        System->>LocalActor: sendToProcess → PostMessage → Router.Handle
        LocalActor-->>Handler: (业务处理，可能再调 Session.Response)
    else 转发到跨集群 Actor
        Handler->>Cluster: ctx.Send(remotePid, "Method", data)
        Cluster->>Cluster: isLocalMessage? → 否
        Cluster->>MQ: transport.Send(nodeId, ActorMessage)
        MQ->>RemoteNode: Publish(subject=nodeId, bytes)
        RemoteNode->>RemoteNode: OnMessage → Unmarshal → system.Send(msg)
        RemoteNode->>RemoteActor: sendToProcess → 目标 Actor 处理
    end
```

---

## 3. 组件职责与关键代码位置

| 环节 | 组件/文件 | 行为 |
|------|-----------|------|
| 入连接 | `gate/gate.go` | `onConnect`: 为每条连接 `Spawn(agent.New(...))`，将 **Pid 存到 conn.Context** |
| 收包 | `gate/gate.go` | `onMessage`: `codec.Decode(data)` 得到 `*protocol.Message`，循环 `process(conn, msg)` |
| 投递到 Agent | `gate/gate.go` | `process`: `system.SubmitTask(pid, func(){ a.OnData(msg) })`，即向该连接对应的 Agent 投递任务 |
| Agent 收包 | `gate/agent/agent.go` | `OnData`: AfterDecode → `session.SetMessage(msg)` → `IHandler.OnRoute(agent, msg.Data)` |
| 路由到 Actor | 业务实现 `IHandler` | `OnRoute` 内通过 `ctx.Send(pid, method, data)` / `ctx.Call(...)` / `ctx.Forward(...)` 发往本节点或跨节点 Actor |
| 本节点发送 | `actor/system.go` | `Send(message)`: `sendToProcess(message.To, message)` → 目标 Process 的 Mailbox |
| 跨节点发送 | `actor/cluster_system.go` | `Send(message)`: 若 `!isLocalMessage` 则 `transport.Send(message.To.NodeId, message)` |
| 集群传输 | `pkg/cluster/cluster.go` | `Send(nodeId, message)`: Marshal 后 `mq.Publish(subject=nodeId, bytes)` |
| 对端收包 | `internal/component/cluster/cluster.go` | `OnMessage`: Unmarshal → `system.Send(msg)` 或 `system.Call(msg)`，本地投递到目标 Actor |

---

## 4. 下行：Actor → Agent → Client（Session 回包）

当业务 Actor（本节点或跨节点）需要向客户端推送数据时，通过 **Session** 的 transport 发回 Agent，再由 Agent 写回连接。

```mermaid
sequenceDiagram
    participant A as Actor
    participant S as Session
    participant T as Transport
    participant Sys as System
    participant Ag as Agent
    participant C as Codec
    participant Cli as Client

    A->>S: Response 或 Push 回包数据
    S->>T: push encodedBin
    T->>T: send HandlerPush to Agent

    alt Agent 在本节点
        T->>Ag: InvokerMessage 直接投递
    else Agent 在其它节点
        T->>Sys: System Send msg
        Sys->>Ag: 经集群 MQ 到目标节点
    end

    Ag->>Ag: HandlerPush Decode then Push
    Ag->>Ag: RunBeforeEncode
    Ag->>C: Encode msg
    Ag->>Cli: entity Send bin
```

- **Session**：`internal/component/gate/session/session.go`，持有 `transport`（目标为 session 绑定的 Agent Pid）。
- **Transport**：`session/transport.go`，`push(bin)` → `send(agent, "HandlerPush", bin)`；同节点用 `InvokerMessage`，跨节点用 `System().Send(msg)`。
- **Agent.HandlerPush**：`agent/agent.go`，解码后调用 `Agent.Push(msg)`，再编码通过 `entity.Send(bin)` 发回客户端。

---

## 5. 数据流简图（仅上行到跨集群 Actor）

```mermaid
flowchart TB
    subgraph client["客户端"]
        A1[Client 发送]
    end

    subgraph gate["Gate"]
        A2[network.OnMessage]
        A3[codec.Decode]
        A4[process: SubmitTask to Agent Pid]
    end

    subgraph agent["Agent"]
        A5[OnData]
        A6[AfterDecode + SetMessage]
        A7[IHandler.OnRoute]
    end

    subgraph routing["路由"]
        A8{目标 Pid 本节点?}
        A9[System.Send 本节点]
        A10[ClusterSystem.Send 跨节点]
    end

    subgraph cluster["集群"]
        A11[transport.Send → MQ.Publish]
        A12[目标节点 Subscribe → OnMessage]
        A13[system.Send/Call → 目标 Actor]
    end

    A1 --> A2 --> A3 --> A4 --> A5 --> A6 --> A7 --> A8
    A8 -->|是| A9
    A8 -->|否| A10 --> A11 --> A12 --> A13
```

---

## 6. 小结

- **Client → Gate**：网络层回调，Gate 按连接绑定的 **Agent Pid** 用 `SubmitTask` 把解码后的 `*protocol.Message` 投递到对应 Agent。
- **Gate → Agent**：每条连接一个 Agent Actor，消息经 **AfterDecode**、**SetMessage** 后交给 **IHandler.OnRoute**。
- **Agent → Actor**：在 OnRoute 中通过 `ctx.Send` / `ctx.Call` / `ctx.Forward` 发往本节点或跨集群 Actor；跨节点时由 **ClusterSystem** 经 **MessageQueue** 发到目标节点，目标节点 **Cluster 组件** 收包后交给本地 **System.Send/Call**，最终进入目标 Actor 的 Mailbox。
- **Actor → Client**：业务通过 **Session.Response/Push** 经 **Transport** 发到对应 Agent（本节点直接投递，跨节点经 System.Send），Agent 的 **HandlerPush** 编码后通过 **entity.Send** 写回客户端。

以上流程对应代码库中的 `internal/component/gate`、`internal/actor`、`internal/component/cluster` 与 `pkg/cluster`。
