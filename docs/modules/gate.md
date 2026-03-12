# internal/component/gate 模块文档

---

## 1. Gate 接口说明

Gate 为网关核心，负责监听地址、管理连接数、在连接建立时为每条连接创建 Agent 并投递解码后的消息。实现 `gateiface.IGate`。

| 方法 | 说明 |
|------|------|
| `Start(ctx context.Context) error` | 启动网关；需先调用 SetSystem、SetAgentHandlerFactory、SetAddress。内部创建 network server、注册 Session 工厂并启动监听。 |
| `Stop(ctx context.Context) error` | 关闭网络服务。 |
| `SetAddress(address string)` | 设置监听地址（如 tcp://127.0.0.1:9000）。 |
| `SetSystem(system iface.ISystem)` | 设置 Actor 系统，Start 前必须调用。 |
| `SetAgentHandlerFactory(f AgentHandlerFactory)` | 设置每连接对应的业务 Handler 工厂（返回 `gateiface.IAgentHandler`），Start 前必须调用。 |
| `AppendOptions(options ...network.Option)` | 设置网络选项（KeepAlive、缓冲区等）。 |
| `SetMaximumOfConn(n int64)` | 设置最大连接数，超过时新连接会被拒绝。 |
| `GetConnectionCount() int64` | 返回当前连接数。 |

---

## 2. Agent 接口说明

Agent 表示每条连接对应的一个 Actor，实现 `gateiface.IAgent` 与 `IRemoteHandler`。业务通过实现 `gateiface.IAgentHandler`（或嵌入 `gateiface.BaseAgentHandlerFactory` 做空实现）接入；Gate 在连接建立时通过 `AgentHandlerFactory()` 创建 Handler 并传给 `agent.New`。

### 2.1 构造与生命周期

| 类型/方法 | 说明 |
|-----------|------|
| `New(entity network.IConnection, handler IAgentHandler) *Agent` | 构造 Agent；handler 为 nil 时使用空实现。由 Gate 在 OnConnect 时调用并 Spawn。 |
| `Agent` | 结构体，嵌 `iface.Actor` 与 `IAgentHandler`，持有 ctx、Session、entity、中间件链。 |
| `OnInit(ctx, params) error` | Actor 初始化时调用，创建 Session 并调用 IAgentHandler.OnInit。 |
| `OnData(msg *protocol.Message) error` | 收到解码后的消息时调用，经 RunAfterDecode 中间件后 SetMessage，再调用 IAgentHandler.OnRoute(agent, msg.Data)。 |
| `OnStop(ctx) error` | Actor 停止时调用，委托 IAgentHandler.OnStop。 |

### 2.2 IAgent 能力（gateiface.IAgent）

| 方法 | 说明 |
|------|------|
| `Context() iface.IContext` | 返回当前 Actor 的 IContext。 |
| `GetEntity() network.IConnection` | 返回绑定的连接。 |
| `GetSession() *session.Session` | 返回 Session。 |
| `AppendMiddleware(...IMiddleware)` | 追加中间件。 |
| `SetMiddleware(chain []IMiddleware)` | 替换整条中间件链。 |
| `GetMiddleware() []IMiddleware` | 返回当前中间件链。 |
| `Push(msg *protocol.Message) error` | 经 RunBeforeEncode 后编码并发送到连接。 |
| `SetValues(values map[string]string) error` | 合并 values 到 Session.Values。 |
| `Shutdown() error` | 关闭连接并关闭当前 Actor。 |

### 2.3 IRemoteHandler（远程 Actor 调用）

| 方法 | 说明 |
|------|------|
| `HandlerPush(ctx, data []byte) error` | 将 data 按协议解码后 Push 到连接。 |
| `HandlerSetValue(ctx, data []byte) error` | 将 data 反序列化为 map 后 SetValues。 |
| `HandlerShutdown(ctx, data []byte) error` | 执行 Shutdown。 |

### 2.4 业务接口 gateiface.IAgentHandler

| 方法 | 说明 |
|------|------|
| `OnInit(agent IAgent) error` | Agent 初始化完成后调用，可做业务初始化、挂中间件等。 |
| `OnRoute(agent IAgent, data []byte) error` | 收到一条消息的 Body 时调用，data 为解码后的包体。 |
| `OnStop(agent IAgent) error` | Agent 即将停止时调用，可做清理。 |

`gateiface.BaseAgentHandlerFactory` 提供上述三者的空实现，业务可嵌入并只重写需要的方法。

---

## 3. Session 接口说明

Session 封装会话数据（嵌 *pb.Session）以及向对端写数据的能力（Response、Push、Close、SyncValues 等）；写操作通过内部 transport 转为对 Agent 的 Actor 调用，最终在持有连接的 Agent 侧执行。

### 3.1 构造与工厂

| 类型/方法 | 说明 |
|-----------|------|
| `Factory` | 实现 iface.ISessionFactory。 |
| `(m *Factory) FromRaw(ctx, raw *pb.Session) iface.ISession` | 用 raw 与 ctx 构造 Session，等价于 `New(raw, ctx)`。 |
| `New(raw *pb.Session, ctx iface.IContext) *Session` | 构造 Session 并绑定 transport。 |

### 3.2 Values 读写

| 方法 | 说明 |
|------|------|
| `SetString(key, value string)` | 设置 Values 中的字符串（不同步到对端）。 |
| `GetString(key string) string` | 从 Values 取字符串。 |
| `SetUint64(key, value)` / `GetUint64(key) uint64` | 以字符串形式存/取 uint64，Get 解析失败返回 0。 |
| `SetInt64(key, value)` / `GetInt64(key) int64` | 以字符串形式存/取 int64，Get 解析失败返回 0。 |
| `SyncValues() error` | 将当前 Values 同步到对端。 |

### 3.3 写对端

| 方法 | 说明 |
|------|------|
| `Response(data []byte) error` | 向对端推送业务响应（沿用当前请求的 Index/Cmd/Act/Tag）。 |
| `ResponseErr(errCode uint16) error` | 推送仅带错误码的无 body 消息。 |
| `Push(cmd, act uint8, data []byte) error` | 按 cmd/act 向对端推送带 body 消息。 |
| `Close() error` | 通知对端关闭连接。 |

### 3.4 当前请求消息与原始 Session

| 方法 | 说明 |
|------|------|
| `SetMessage(msg *protocol.Message)` | 设置当前请求消息并写入 Values（base64），供集群序列化后 GetMessage 使用。 |
| `GetMessage() *protocol.Message` | 返回当前请求消息（必要时从 Values 解码）。 |
| `GetAgent() *iface.Pid` | 返回 Session 绑定的 Agent Pid。 |
| `Raw() *pb.Session` | 返回 *pb.Session（会先把当前 msg 写回 Values）。 |

---

## 4. Message 接口说明（protocol）

Message 表示网关二进制协议中的一条消息，由 Head + Data 组成；Head 字段通过 Get/Set 访问。

### 4.1 类型与构造

| 类型/函数 | 说明 |
|-----------|------|
| `Message` | 结构体：*Head + Data []byte。 |
| `Head` | 协议头结构体，字段见下方 Get/Set。 |
| `New(cmd, act uint8, data []byte) *Message` | 构造消息，Len 由 codec 按 Data 长度写入。 |
| `NewData(data []byte) *Message` | 构造 (0,0) 纯数据消息。 |
| `NewErr(err uint16) *Message` | 构造仅带错误码的消息。 |
| `NewDecoded(bodyLen, cmd, act, errCode, index, tag uint8/uint32/uint16, data []byte) *Message` | codec 解码时构造完整消息，包外一般不需直接调用。 |

### 4.2 Message 方法

| 方法 | 说明 |
|------|------|
| `Copy(old *Message)` | 从 old 复制 Cmd、Act、Index、Tag，用于回包。 |
| `ID() uint16` | 返回 Cmd<<8+Act。 |

### 4.3 Head 方法

| 方法 | 说明 |
|------|------|
| `GetLen()/SetLen(v)` | 包体长度。 |
| `GetCmd()/SetCmd(v)` | 命令。 |
| `GetAct()/SetAct(v)` | 动作。 |
| `GetError()/SetError(v)` | 错误码。 |
| `GetIndex()/SetIndex(v)` | 序号。 |
| `GetTag()/SetTag(v)` | 标签（业务或中间件使用）。 |
| `Clone() *Head` | 返回 Head 副本。 |

### 4.4 工具函数

| 函数 | 说明 |
|------|------|
| `CmdAct(cmd, act uint8) uint16` | 将 cmd、act 合并为 16 位 ID。 |
| `ParseId(msgId uint16) (cmd, act uint8)` | 将 msgId 拆成 cmd、act。 |

---

## 5. 中间件说明

中间件实现 `gateiface.IMiddleware`，在**解码后**（AfterDecode）和**编码前**（BeforeEncode）对 `*protocol.Message` 做修改或拦截。链由 Agent 维护，通过 `RunAfterDecode` / `RunBeforeEncode` 顺序执行；链中 nil 被跳过，任一返回 error 或 nil msg 即终止后续处理。

### 5.1 链的执行方式

- **RunAfterDecode(chain, agent, msg)**：codec 解码完成后、交给业务 OnRoute 之前执行；用于解压、解密、限流、打日志等。
- **RunBeforeEncode(chain, agent, msg)**：业务调用 Push 时、codec 编码之前执行；用于压缩、加密、打日志等。
- 中间件可返回新的 Message（如 Clone Head 并改 Tag），或返回 nil 表示丢弃/已自行响应，不再继续。

---

### 5.2 Log（日志）

**作用**：在收包与发包时打 Debug 日志，便于排查；不修改消息内容。

**实现**：

- **AfterDecode**：若 msg 非 nil，用 `glog.Debug("gate.recv", cmd, act, len, tag)` 记录后原样返回 msg。
- **BeforeEncode**：若 msg 非 nil，用 `glog.Debug("gate.send", cmd, act, len(Data), tag)` 记录后原样返回 msg。

不读写 Head 或 Data，仅透传。

**使用**：`middleware.NewLog()` 得到 `*Log`，在 Agent 的 OnInit 中 `AppendMiddleware(NewLog())` 即可。

---

### 5.3 Compress（压缩）

**作用**：对消息 Body 做 gzip 压缩/解压，减少带宽；通过协议头 Head.Tag 的标记位表示当前 Data 是否为压缩后内容，与加密等可共用 Tag 不同位。

**实现**：

- **标记位**：Head.Tag 的某一位（如 1<<0）表示 “Data 已 gzip 压缩”；解压后清除该位。
- **BeforeEncode（发方向）**：  
  - 若 Body 为空或长度小于配置的 `minLen`（minLen>0 时）则不解压，原样返回。  
  - 否则对 `msg.Data` 做 gzip 压缩，用 `Head.Clone()` 得到新头并设置 Tag 压缩位，返回新 Message(Head, 压缩后 Data)。
- **AfterDecode（收方向）**：  
  - 若 Tag 未带压缩位或 Data 为空，原样返回。  
  - 否则用 `gzip.NewReader` 解压 Data，新头清除压缩位，返回新 Message(Head, 解压后 Data)。  
  - 解压失败返回错误，链终止。

**使用**：`middleware.NewCompress(minLen)`，minLen 为最小压缩长度，0 表示一律压缩；每连接可挂一个实例。

---

### 5.4 Encrypt（加密）

**作用**：在连接上做一次密钥交换后，对后续消息 Body 做按位异或（XOR）加解密，防止明文传输；与压缩可叠加（先解密再解压等顺序由链顺序决定）。

**实现**：

- **约定**：cmd=0、act=0 的报文视为密钥交换专用：客户端发 clientKey，服务端回 serverKey；密钥交换报文不参与 XOR。
- **密钥派生**：双方用 `deriveKey(serverKey, clientKey) = serverKey || clientKey` 得到相同对称密钥（字节拼接），用于后续 XOR。
- **服务端**：  
  - `NewEncrypt()` 随机生成 32 字节 serverKey。  
  - AfterDecode 收到 (0,0, clientKey) 时：派生 `derivedKey = deriveKey(serverKey, clientKey)`，并通过 `agent.Push(New(0,0, serverKey))` 直接回包给对端，然后返回 nil 使该条消息不再进入业务。  
  - 其他报文：若已派生密钥且 Tag 带加密位，则对 Data 做 XOR 解密并清除 Tag 加密位。
- **客户端**：  
  - `NewEncryptClient(clientKey)` 保存 clientKey。  
  - AfterDecode 收到 (0,0, serverKey) 时：派生 `derivedKey = deriveKey(serverKey, clientKey)`，并返回 msg 交给业务（可选由业务忽略或做状态更新）。  
  - 其他报文解密逻辑同服务端。
- **BeforeEncode**：对非 (0,0) 且 Data 非空的消息，用 derivedKey 做 XOR 加密，并设置 Tag 加密位。

**使用**：每连接一个实例；服务端 `NewEncrypt()`，客户端 `NewEncryptClient(clientKey)`；密钥交换完成后的报文才会被加解密。

---

### 5.5 RateLimit（限流）

**作用**：基于令牌桶对收包做限流，支持按“整条连接”或按“单连接上的某种消息 ID”限制频率；超限时终止链并返回错误，避免业务继续处理。

**实现**：

- **依赖**：`golang.org/x/time/rate` 的 `rate.Limiter`（令牌桶）。
- **两种模式**：  
  - **按连接**：一个实例一个 Limiter，限制该连接上所有消息的整体速率。  
    - `NewRateLimitForConnection(limit, burst)` 创建；每连接一个实例。  
  - **按消息 ID**：一个实例只限制**一种**消息 ID（构造时传入 `messageId`，即 Cmd<<8|Act）。  
    - 内部用 `map[uint16]*rate.Limiter` 按 msg.ID() 缓存 Limiter；仅当 `msg.ID() == messageId` 时才会对该条消息做 Allow()，其它 ID 直接放行。  
    - `NewRateLimitForMessageID(limit, burst, messageId)` 创建；每连接、每种要限流的 ID 各一个实例。
- **AfterDecode**：对当前 msg 取对应 Limiter（按连接则用唯一桶，按 ID 则用 limiterForID(msg.ID())），调用 `Allow()`；若不允许则返回 `ErrRateLimitExceeded`，链终止。
- **BeforeEncode**：不做限流，原样返回 msg。

**使用**：在 Agent 的 OnInit 中按需追加；按连接限流用 `NewRateLimitForConnection`，按某消息 ID 限流用 `NewRateLimitForMessageID`，超限后调用方会收到限流错误。

---

## 6. 消息结构说明（二进制协议）

网关层使用**大端序**、**固定头 + 变长 Body** 的二进制协议，由 `gate/codec` 的 `Encode`/`Decode` 编解码。

### 6.1 整体布局

- **头（Head）**：固定 13 字节。  
- **体（Body）**：长度由头中 Len 指定，单条 Body 最大 1MB。

### 6.2 头字段（13 字节）

| 字段 | 类型 | 字节数 | 说明 |
|------|------|--------|------|
| Len | uint32 | 4 | 包体（Data）长度，大端序。 |
| Cmd | uint8 | 1 | 命令。 |
| Act | uint8 | 1 | 动作。 |
| Error | uint16 | 2 | 错误码，大端序。 |
| Index | uint32 | 4 | 序号，大端序；回包时可沿用请求的 Index。 |
| Tag | uint8 | 1 | 标签，供业务或中间件使用（如压缩/加密标记）。 |

### 6.3 编解码

- `codec.Encode(msg *protocol.Message) ([]byte, error)`：将 Message 编码为上述格式；msg 为 nil 或 Body 超长返回错误。  
- `codec.Decode(buf []byte) (*protocol.Message, int, error)`：从 buf 解出一个完整包，返回消息与消费字节数；数据不足时返回 (nil, 0, nil)。
