# Actor 包与 Session 解耦方案建议

本文档从架构角度说明如何在 `actor` 包中解耦对 `session` 的依赖，**仅作建议与设计说明，不直接修改业务逻辑代码**。实施时可按步骤落地。

---

## 一、当前耦合点

| 位置 | 依赖内容 | 说明 |
|------|----------|------|
| **actor/context.go** | `gate/session`、`session.New`、`sessionTransport` | `execHandler` 中根据 `msg.GetSession()` 构造可写 Session，并依赖 `session.ITransport` 的实现（当前为 actor 内的 `sessionTransport`） |
| **actor/router.go** | 会话参数的反射类型 | 路由识别「会话消息」时若用具体类型 `*session.Session`，则 router 必须 import `gate/session` |
| **gate/agent.go** | `*session.Session` | IAgentHandler / IAgent 的 Push、Shutdown、OnData、SetValue 等使用具体类型 `*session.Session`，与 iface 中的 `ISession` 不一致 |

**目标**：让 `actor` 包**不 import** `internal/gate/session`，仅依赖 `iface` 中的会话抽象；Session 的「数据 + 可写能力」由上层（如 gate）通过接口注入。

---

## 二、核心思路：Session 工厂 + 接口类型

- **actor 只认「接口」**：消息里携带的会话数据在 iface 中已有（如 `*iface.Session`）；actor 在执行业务前，需要的是「能 Response/Push/Close 的会话对象」即 `iface.ISession`。
- **谁构造 ISession 由上层决定**：actor 不直接调用 `session.New(...)`，而是通过一个「工厂」把 `*iface.Session` 变成 `iface.ISession`。工厂由 System 在创建进程时注入到 Context。
- **传输逻辑上移到 gate**：当前在 actor 包内的 `sessionTransport`（把 Session 的 Response/Push/Close 转成发往 Agent 的 Actor 消息）可以移到 gate 包实现，并作为工厂内部使用的 `session.ITransport`。actor 只依赖「能从 raw 构造 ISession」的接口，不依赖具体实现。

这样：

- actor 仅依赖：`iface.ISession`、`iface.ISessionFactory`（及现有 iface 定义）。
- gate 依赖：`iface`、`gate/session`，并实现 `ISessionFactory` 和 `session.ITransport`。

---

## 三、接口与职责划分

### 3.1 在 iface 中增加 Session 工厂接口（若尚未存在）

建议在 `iface` 中定义：

- **ISessionFactory**：`FromRaw(ctx IContext, raw *Session) ISession`
  - 含义：根据当前上下文和消息中的原始 Session 数据，构造一个可写的 `ISession`。
  - 实现方：gate 包（内部可调用 `session.New(raw, transport)`，transport 由 gate 提供）。

这样 actor 在 `execHandler` 中只需：

- 若 `msg.GetSession() != nil` 且 `ctx.sessionFactory != nil`，则 `s = ctx.sessionFactory.FromRaw(ctx, raw)`，再把 `s` 交给 router。
- 不出现 `session` 包或 `sessionTransport` 的引用。

### 3.2 actor 包内改动建议

1. **actorContext**
   - 增加可选字段：`sessionFactory iface.ISessionFactory`。
   - 在 `execHandler` 中：用 `sessionFactory.FromRaw(ctx, raw)` 得到 `iface.ISession`，不再调用 `session.New` 或使用 `sessionTransport`。
   - 删除对 `internal/gate/session` 的 import；删除 `sessionTransport` 类型及其 `Send`/`newMessage` 实现（或将这些实现迁到 gate）。

2. **System / spawn**
   - System 提供**可选的** Session 工厂注入方式（例如字段 `sessionFactory iface.ISessionFactory` + `SetSessionFactory(f)`，或通过 Option 注入）。
   - 在 `spawn` 创建 `actorContext` 时，若当前 System 支持提供工厂（例如实现 `SessionFactory() iface.ISessionFactory`），则把该工厂赋给 `ctx.sessionFactory`。
   - 不把 `ISessionFactory` 写进 `iface.ISystem` 亦可：可用仅在 actor 包内可见的「带 SessionFactory 的 System」接口，由 `actor.System` 实现，避免污染全局 iface。

3. **Router**
   - 识别「会话消息」时，按**接口类型**判断参数是否为实现 `iface.ISession` 的类型（例如 `paramType.Implements(typeOfISession)`），而不是写死 `*session.Session`。
   - 这样 router 不再依赖 `gate/session`，handler 签名可以是 `(ctx, session iface.ISession, ...)`，具体实现仍可以是 `*session.Session`。

### 3.3 gate 包职责建议

1. **实现 ISessionFactory**
   - 类型例如：`type sessionFactory struct{}`，实现 `FromRaw(ctx iface.IContext, raw *iface.Session) iface.ISession`。
   - 内部：`raw` 深拷贝后，用 `session.New(raw, transport)` 返回，其中 `transport` 为 gate 内实现的 `session.ITransport`。

2. **实现 session.ITransport（原 sessionTransport 逻辑）**
   - 将当前 actor 包中「根据 ctx 把 Session 操作转成发往 Agent 的 Actor 消息」的逻辑移到 gate 包（例如 `gateSessionTransport` 持有 `iface.IContext`，实现 `Send(session, route, payload)`：序列化、构造 ActorMessage、根据 session.GetAgent() 与 ctx.ID() 决定本进程投递或 System.Send）。
   - 这样「如何把 Session 的 Push/Close/Response 变成对 Agent 的调用」完全由 gate 决定，actor 不感知。

3. **应用层装配**
   - 在创建 System 并交给 Gate 使用之前，调用 `system.SetSessionFactory(gate.NewSessionFactory())`（或等价方式），这样带 Gate 的节点才会在处理消息时构造可写 Session；纯 Actor 节点可不设置，`sessionFactory == nil` 时行为与「无 Session 能力」一致。

---

## 四、Gate / Agent 侧类型建议

- **IAgentHandler / IAgent**：若希望 gate 与 actor 完全通过接口协作，可考虑将 `OnData`、`Push`、`Shutdown`、`SetValue` 等的 session 参数类型改为 `iface.ISession`；实现里若需要访问具体能力再断言为 `*session.Session`。这样 actor 侧路由传下来的 `iface.ISession` 可直接传入，无需 gate 再依赖「只有 session 包才有的类型」。
- **entity.Context() 存什么**：连接上可继续存「会话数据」即可（例如 `*iface.Session` 或 gate 内定义的只读视图）；不要求必须存 `*session.Session`。需要往连接写回时，由 Agent 的 Push/Shutdown 等通过 `iface.ISession` 拿到 Agent Pid / EntityId 等，再查连接表写回。

---

## 五、实施顺序建议（仅作顺序参考，不直接改代码）

1. **iface**：补充 `ISessionFactory`（若尚未有），并确认 `ISession` 已包含业务所需方法（如 Response、Push、Close、SetValue 等）。
2. **actor**：在 context 中增加 `sessionFactory` 字段；在 spawn 中从 System 注入；`execHandler` 改为通过工厂得到 `ISession`；移除对 `gate/session` 的引用及 `sessionTransport` 的实现。
3. **actor**：Router 中会话参数识别改为基于 `iface.ISession` 的反射判断。
4. **gate**：实现 `ISessionFactory` 和 `session.ITransport`，并在应用启动时把工厂注入到 System。
5. **gate/agent**（可选）：将 IAgent 等接口的 session 参数统一为 `iface.ISession`，内部再按需断言。

---

## 六、效果小结

- **actor 包**：不再依赖 `internal/gate/session`，只依赖 `iface` 的 Session 与 Session 工厂抽象；可单独复用到「无 Gate、无网络 Session」的场景。
- **session 能力**：由上层通过 `ISessionFactory` 注入，有则构造可写 Session，无则 nil，行为清晰。
- **测试**：actor 单测可 mock `ISessionFactory` 和 `ISession`，无需依赖真实 gate/session 实现。

以上为「仅建议、不直接改业务代码」的 actor–session 解耦方案，实施时可按项目节奏分步替换。
