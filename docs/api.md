# GAS API 文档

本文档描述 gas 库对外暴露的主要接口与类型，与当前代码保持一致。

---

## Node 接口

Node 为进程内节点抽象：持有 Member 信息、序列化器与组件管理器，对外提供 System、Cluster、Profile。通过 `Startup` 完成配置加载、组件注册与启动、集群注册，并阻塞等待退出信号后做优雅关闭。

**创建与配置**

| 类型/方法 | 说明 |
|-----------|------|
| `node.New() *Node` | 创建节点实例；默认配置路径为 `config.yml`，类型为 `yaml`。 |
| `(*Node)SetConfigPath(path string)` | 设置配置文件路径。 |
| `(*Node)SetConfigType(typ string)` | 设置配置文件类型（如 `yaml`、`json`）。 |

**INode 接口（含 IMember 与 component.IManager[INode]）**

| 方法 | 说明 |
|------|------|
| `(*Node)Info() *iface.Member` | 返回节点 Member。 |
| `(*Node)System() iface.ISystem` | 返回 Actor 系统（从已注册组件中获取）；未注册时返回 nil。 |
| `(*Node)Cluster() cluster.ICluster` | 返回集群接口（从已注册组件中获取）；未注册时返回 nil。 |
| `(*Node)Profile() iface.IProfile` | 返回配置加载器（从已注册组件中获取）。 |
| `(*Node)Serializer() serializer.ISerializer` | 返回当前序列化器（`pkg/lib/serializer`）。 |
| `(*Node)SetSerializer(ser serializer.ISerializer)` | 设置序列化器。 |
| `(*Node)SetPanicHook(hook func(zapcore.Entry))` | 设置 panic 时回调（如 logger 组件会使用）。 |
| `(*Node)Startup(comps ...component.IComponent[INode]) error` | 启动节点：注册并启动内置组件（profile、logger、cluster、system）与传入的 comps、在集群中注册本节点、阻塞等待退出信号后停止所有组件并收尾。返回时即已关闭完成。 |

**组件管理器（INode 内嵌 component.IManager[INode]）**

| 方法 | 说明 |
|------|------|
| `Register(comp component.IComponent[INode]) error` | 注册组件（需在 Startup 前完成）。 |
| `GetComponent(name string) IComponent[INode]` | 按名称获取组件。 |
| `GetComponentNames() []string` | 返回已注册组件名称列表。 |

---

## Actor 接口

### 包级导出（internal/actor）

| 函数/方法 | 说明 |
|-----------|------|
| `NewSystem(selfNodeID uint64, ser serializer.ISerializer) *System` | 创建单节点 Actor 系统。 |
| `NewClusterSystem(selfNodeID uint64, ser serializer.ISerializer, transport cluster.ICluster) *ClusterSystem` | 创建集群版 Actor 系统，跨节点消息经 transport 转发。 |
| `NewRouter() iface.IRouter` | 创建路由器。 |
| `RegisterRouter(actor iface.IActor) iface.IRouter` | 为指定 actor 类型获取或创建并注册 Router。 |
| `NewDefaultDispatcher(throughput int) IDispatcher` | 创建默认 goroutine 调度器。 |
| `NewSynchronizedDispatcher(throughput int) IDispatcher` | 创建同步调度器。 |

### ISystem

| 方法 | 说明 |
|------|------|
| `(*System)NodeId() uint64` | 本节点 ID。 |
| `(*System)NextID() uint64` | 生成下一个 ActorId。 |
| `(*System)Serializer() serializer.ISerializer` | 序列化器。 |
| `(*System)SessionFactory() iface.ISessionFactory` / `SetSessionFactory(f iface.ISessionFactory)` | Session 工厂（可选，由 gate 等上层注入）。 |
| `(*System)Spawn(actor iface.IActor, args ...interface{}) *Pid` | 创建并注册新进程，投递 OnInit 后返回 Pid。 |
| `(*System)Register(ctx IContext) error` | 将 Context 注册到 IdDict。 |
| `(*System)Unregister(ctx IContext) error` | 从 IdDict 删除并 Unname。 |
| `(*System)Named(ctx IContext) error` / `Unname(ctx IContext) error` | 为 ctx 注册/注销其当前名字（GetName()）。 |
| `(*System)SubmitTask(pid *Pid, task Task) error` | 异步投递任务到进程。 |
| `(*System)SubmitTaskAndWait(pid *Pid, task Task, timeout time.Duration) error` | 投递任务并等待完成或超时。 |
| `(*System)SendMessage(message *ActorMessage) error` | 异步发送已构造的 ActorMessage（仅本节点）。 |
| `(*System)CallMessage(message *ActorMessage) (data []byte, err error)` | 同步发送 ActorMessage 并等待响应，超时由 message.Deadline 决定。 |
| `(*System)Send(from, to *Pid, methodName string, request interface{}) error` | 便捷：按 from/to/methodName/request 构造消息并异步发送。 |
| `(*System)Call(from, to *Pid, methodName string, request interface{}, reply interface{}, timeout time.Duration) error` | 便捷：构造消息同步调用，响应反序列化到 reply。 |
| `(*System)GetProcess(ref interface{}) IProcess` | ref 可为 string（名字）、uint64（ActorId）、*Pid。 |
| `(*System)GetAllProcesses() []IProcess` | 返回所有已注册进程。 |
| `(*System)ShutdownProcess(pid *Pid) error` | 关闭指定进程。 |
| `(*System)Shutdown() error` | 关闭系统（不等待所有进程完全退出）。 |

### ClusterSystem

在 System 基础上重写跨节点语义：

| 方法 | 说明 |
|------|------|
| `(*ClusterSystem)SendMessage(message *ActorMessage) error` | 本节点走 System.SendMessage，跨节点走 transport.Send(nodeId, message)。 |
| `(*ClusterSystem)CallMessage(message *ActorMessage) (data []byte, err error)` | 本节点走 System.CallMessage，跨节点走 transport.Call 并反序列化 Response。 |
| `(*ClusterSystem)Send(from, to *Pid, methodName string, request interface{}) error` | 同 SendMessage 的本地/跨节点分流。 |
| `(*ClusterSystem)Call(from, to *Pid, methodName string, request interface{}, reply interface{}, timeout time.Duration) error` | 同 CallMessage 的本地/跨节点分流。 |

### IContext

| 方法 | 说明 |
|------|------|
| `ID() *Pid` | 当前进程 Pid。 |
| `Serializer() serializer.ISerializer` | 序列化器。 |
| `Named(name string) error` / `Unname() error` | 注册/注销当前进程名字。 |
| `GetName() string` | 当前注册的名字。 |
| `Actor() IActor` | 当前 Actor。 |
| `Message() *ActorMessage` | 当前正在处理的消息。 |
| `Process() IProcess` | 当前进程。 |
| `System() ISystem` | 所属 System。 |
| `SetCallTimeout(timeout time.Duration)` | 设置 Call 默认超时。 |
| `Send(to *Pid, methodName string, request interface{}) error` | 异步发送。 |
| `Call(to *Pid, methodName string, request interface{}, reply interface{}) error` | 同步调用（使用 SetCallTimeout 设置的超时）。 |
| `SendMessage(message *ActorMessage) error` | 发送已构造消息。 |
| `CallMessage(message *ActorMessage) (data []byte, err error)` | 同步发送并等待响应。 |
| `ForwardMessage(pid *Pid, methodName string) error` | 转发当前消息到目标进程的指定方法。 |
| `AfterFunc(duration time.Duration, task Task) *timer.Timer` | 延迟执行任务。 |
| `Shutdown() error` | 关闭当前进程。 |

### IActor

| 方法 | 说明 |
|------|------|
| `OnInit(ctx IContext, params []interface{}) error` | 进程创建后首次投递的任务。 |
| `OnMessage(ctx IContext, msg interface{}) error` | 收到消息时调用。 |
| `OnStop(ctx IContext) error` | 进程即将停止时调用。 |

`iface.Actor` 为空实现，可嵌入后只重写需要的方法。

### 消息与 Pid（internal/iface）

| 类型/函数 | 说明 |
|-----------|------|
| `NewActorMessage(from, to *Pid, methodName string, data []byte) *ActorMessage` | 构造异步消息；可再设置 Async、Deadline、SetResponse。 |
| `NewResponse(data []byte, err error) *Response` | 构造 RPC 响应。 |
| `NewPid(nodeId, actorId uint64) *Pid` | 按 ID 构造 Pid。 |
| `NewPidWithName(name string, nodeId uint64) *Pid` | 按名字与节点 ID 构造 Pid。 |
| `EqualPid(o, other *Pid) bool` | 逻辑相等判断。 |

---

## Cluster 接口（pkg/cluster）

`ICluster` 封装服务发现与消息队列，提供 Run、订阅/收发、注册与查询、选路、Watch、Shutdown。

| 方法 | 说明 |
|------|------|
| `Run(ctx context.Context) error` | 启动内部消息队列与服务发现。 |
| `Subscribe(nodeId uint64, subscriber messageQue.ISubscriber) (messageQue.ISubscription, error)` | 以 nodeId 为 subject 订阅，收到消息时回调 subscriber.OnMessage。 |
| `Send(nodeId uint64, message interface{}) error` | 将 message 序列化后发送到 nodeId 对应 subject（单向）。 |
| `Call(nodeId uint64, message interface{}, timeout time.Duration) ([]byte, error)` | 序列化后请求 nodeId，阻塞至收到回复或超时，返回反序列化前的 data。 |
| `Register(member *discovery.Member) error` | 注册成员到服务发现。 |
| `Deregister(memberId uint64) error` | 从服务发现注销成员。 |
| `Update(member *discovery.Member) error` | 更新成员信息。 |
| `Select(tag string, strategy RouteStrategy) (uint64, error)` | 按 tag 取成员列表，用 strategy 选一个成员，返回其 GetID()；无成员或选路失败返回 ErrNotFoundMember。 |
| `GetById(memberId uint64) *discovery.Member` | 按 ID 查询成员。 |
| `GetByKind(kind string) map[uint64]*discovery.Member` | 按 Kind 查询成员映射。 |
| `GetByTag(tag string) []*discovery.Member` | 按 Tag 查询成员列表。 |
| `GetAll() map[uint64]*discovery.Member` | 返回所有成员。 |
| `Watch(kind string, handler discovery.ServiceChangeHandler)` | 注册拓扑变更回调。 |
| `Unwatch(kind string, handler discovery.ServiceChangeHandler)` | 取消拓扑变更回调。 |
| `Shutdown(ctx context.Context) error` | 关闭（先服务发现再消息队列）。 |

---

## Profile 接口（IProfile）

由 `internal/component/profile` 实现，配置文件格式为 YAML（或 viper 支持的其他格式）。

| 方法 | 说明 |
|------|------|
| `Get(key string, cfg interface{}) error` | 将配置中 key 对应内容反序列化到 cfg。 |
| `GetCluster() *cluster.Config` | 读取 `cluster` 配置并填充默认值。 |
| `GetLogger() *glog.Config` | 读取 `logger` 配置并填充默认值。 |
| `IsSingleNodeMode() bool` | 读取顶层布尔配置 `single-node`。 |

---

## Network 接口（pkg/network）

支持 `tcp://`、`udp://`、`ws://`、`wss://`。

### IServer

| 方法 | 说明 |
|------|------|
| `NewServer(handler IHandler, protoAddr string, option ...Option) (IServer, error)` | 根据 protoAddr 解析协议并创建对应 Server；handler 不可 nil。 |
| `Start() error` | 启动监听。 |
| `Addr() string` | 监听地址。 |
| `Shutdown(ctx context.Context)` | 优雅关闭。 |

### IConnection

| 方法 | 说明 |
|------|------|
| `ID() int64` | 连接唯一 ID。 |
| `Send(msg []byte) error` | 发送。 |
| `LocalAddr()` / `RemoteAddr() string` | 本地/远端地址。 |
| `IsStop() bool` | 是否已关闭。 |
| `Type() ConnType` | Accept / Connect。 |
| `Context() interface{}` / `SetContext(interface{})` | 用户数据（如 Gate 存 Agent Pid）。 |
| `SetReadBuffer(bytes int)` / `SetWriteBuffer(bytes int)` / `SetLinger` / `SetNoDelay` / `SetTCPKeepAlive` | 选项。 |
| `Close(err error) error` | 关闭，可重复调用。 |

### IHandler

| 方法 | 说明 |
|------|------|
| `OnConnect(conn IConnection) error` | 连接建立；返回错误则关闭连接。 |
| `OnMessage(conn IConnection, data []byte) (int, error)` | 收到数据；返回已处理字节数与错误。 |
| `OnClose(conn IConnection, err error)` | 连接关闭。 |

---

## Log（pkg/glog）

基于 zap 的全局日志，支持结构化字段与格式化接口。

| 函数/方法 | 说明 |
|-----------|------|
| `Init(cfg *Config)` | 初始化全局 logger（nil 时使用默认配置）。 |
| `Stop() error` | Sync 当前 logger 与 sugared，进程退出前调用以刷盘。 |
| `SetLogLevel(level zapcore.Level)` | 设置全局日志级别。 |
| `GetLevel() zapcore.Level` | 返回当前全局日志级别。 |
| `WithOptions(opts ...zap.Option)` | 在现有 logger 上应用 zap.Option，并替换包级 logger。 |
| `Debug(msg string, fields ...zap.Field)` / `Info` / `Warn` / `Error` / `Panic` / `Fatal` | 结构化输出。 |
| `Debugf(template string, args ...interface{})` / `Infof` / `Warnf` / `Errorf` / `DPanicf` / `Panicf` / `Fatalf` | 格式化输出。 |

---

## Gate（internal/component/gate）

Gate 为网关核心，负责监听、管理连接数、为每条连接创建 Agent 并投递解码后的消息。实现 `gateiface.IGate`。

### IGate 接口

| 方法 | 说明 |
|------|------|
| `Start(ctx context.Context) error` | 启动网关；需先调用 SetSystem、SetAddress；业务 Handler 通过 gate.NewComponent(handler) 注入。 |
| `Stop(ctx context.Context) error` | 关闭网络服务。 |
| `SetAddress(address string)` | 设置监听地址（如 `tcp://127.0.0.1:9000`）。 |
| `SetSystem(system iface.ISystem)` | 设置 Actor 系统，Start 前必须调用。 |
| `AppendOptions(options ...network.Option)` | 追加网络选项（KeepAlive、缓冲区等）。 |
| `SetMaximumOfConn(n int64)` | 设置最大连接数，超过时新连接会被拒绝。 |
| `GetConnectionCount() int64` | 返回当前连接数。 |

### 网关组件

| 类型/方法 | 说明 |
|-----------|------|
| `gate.NewComponent(handler gateiface.IBusinessHandler) *Component` | 创建网关组件；handler 为每连接业务逻辑（OnInit/OnRoute/OnStop），配置从 profile 的 `gate` 段读取并在 Start 时应用。 |

### IBusinessHandler（业务侧实现）

| 方法 | 说明 |
|------|------|
| `OnInit(agent IAgent) error` | Agent 初始化完成后调用。 |
| `OnRoute(agent IAgent, data []byte) error` | 收到一条消息的 Body 时调用，data 为解码后的包体。 |
| `OnStop(agent IAgent) error` | Agent 即将停止时调用。 |

`gateiface.AgentHandler` 为空实现，可嵌入后只重写需要的方法。

### IAgent 接口（gate 暴露的 Agent 能力）

| 方法 | 说明 |
|------|------|
| `Context() iface.IContext` | 当前 Actor 的 IContext。 |
| `GetEntity() network.IConnection` | 绑定的连接。 |
| `GetSession() *session.Session` | Session。 |
| `AppendMiddleware(...IMiddleware)` | 追加中间件。 |
| `SetMiddleware(chain []IMiddleware)` | 替换整条中间件链。 |
| `GetMiddleware() []IMiddleware` | 返回当前中间件链。 |
| `Push(msg *protocol.Message) error` | 经 BeforeEncode 后编码并发送到连接。 |
| `SetValues(values map[string]string) error` | 合并 values 到 Session.Values。 |
| `Shutdown() error` | 关闭连接并关闭当前 Actor。 |

### Session 工厂与 iface.ISession

| 类型/方法 | 说明 |
|-----------|------|
| `session.Factory` | 实现 iface.ISessionFactory。 |
| `(Factory)FromRaw(ctx IContext, raw *pb.Session) iface.ISession` | 用 raw 与 ctx 构造 Session，等价于 session.New(raw, ctx)。 |

**iface.ISession**（actor 层可见）：`GetId() int64`、`Raw() *pb.Session`、`SetString`/`GetString`、`SetUint64`/`GetUint64`、`SetInt64`/`GetInt64`。

### Session（gate/session.Session，连接上下文）

在实现 iface.ISession 基础上增加写能力（Response、Push、Close 等），通过 transport 下发到对端。

| 方法 | 说明 |
|------|------|
| `SetString(key, value string)` / `GetString(key string) string` | Values 中字符串。 |
| `SetUint64(key string, value uint64)` / `GetUint64(key string) uint64` | Values 中 uint64（字符串形式）。 |
| `SetInt64(key string, value int64)` / `GetInt64(key string) int64` | Values 中 int64。 |
| `SyncValues() error` | 将当前 Values 同步到对端。 |
| `Response(data []byte) error` | 向对端推送业务响应（沿用当前请求的 Index/Cmd/Act/Tag）。 |
| `ResponseErr(errCode uint16) error` | 推送仅带错误码的无 body 消息。 |
| `Push(cmd, act uint8, data []byte) error` | 按 cmd/act 向对端推送带 body 消息。 |
| `Close() error` | 通知对端关闭连接。 |
| `SetMessage(msg *protocol.Message)` / `GetMessage() *protocol.Message` | 当前请求消息（供集群序列化等使用）。 |
| `GetAgent() *iface.Pid` | Session 绑定的 Agent Pid。 |
| `Raw() *pb.Session` | 返回 *pb.Session。 |

---

## Message（internal/component/gate/protocol）

| 类型/函数 | 说明 |
|-----------|------|
| `New(cmd, act uint8, data []byte) *Message` | 构造消息，Len 由 codec 按 Data 长度写入。 |
| `NewData(data []byte) *Message` | 构造 (0,0) 纯数据消息。 |
| `NewErr(err uint16) *Message` | 构造仅带错误码的消息。 |
| `NewDecoded(bodyLen uint32, cmd, act uint8, errCode uint16, index uint32, tag uint8, data []byte) *Message` | codec 解码时构造完整消息。 |
| `(*Message)Copy(old *Message)` | 从 old 复制 Cmd、Act、Index、Tag，用于回包。 |
| `(*Message)ID() uint16` | 返回 Cmd<<8+Act。 |
| `(*Head)GetLen()/SetLen(v)` / `GetCmd()/SetCmd(v)` / `GetAct()/SetAct(v)` / `GetError()/SetError(v)` / `GetIndex()/SetIndex(v)` / `GetTag()/SetTag(v)` | Head 访问器。 |
| `(*Head)Clone() *Head` | 返回 Head 副本。 |
| `CmdAct(cmd, act uint8) uint16` | 将 cmd、act 合并为 16 位 ID。 |
| `ParseId(msgId uint16) (cmd, act uint8)` | 将 msgId 拆成 cmd、act。 |
