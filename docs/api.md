

## Node 接口

Node 为进程内节点抽象：持有 Member 信息、序列化器与组件管理器，对外提供 System、Cluster。通过 `Startup` 完成配置加载、组件注册与启动、集群注册，并阻塞等待退出信号后做优雅关闭。

| 类型/方法                                                       | 说明 |
|-------------------------------------------------------------|------|
| `New(path string) *Node`                                    | 创建节点实例；path 为配置文件路径。 |
| `(*Node)Info() *iface.Member`                               | 返回节点 Member。 |
| `(*Node)System() iface.ISystem`                                    | 返回 Actor 系统（从已注册组件中获取）；未注册时返回 nil。 |
| `(*Node)Cluster() cluster.ICluster`                                | 返回集群接口（从已注册组件中获取）；未注册时返回 nil。 |
| `(*Node)Serializer() lib.ISerializer`                              | 返回当前序列化器。 |
| `(*Node)SetSerializer(ser lib.ISerializer)`                        | 设置序列化器。 |
| `(*Node)SetPanicHook(hook func(zapcore.Entry))`                    | 设置 panic 时回调。 |
| `(*Node)Startup(comps ...component.IComponent[iface.INode]) error` | 启动节点：初始化 profile、加载 node 配置、注册并启动内置组件与 comps、在集群中注册本节点、阻塞等待退出信号后停止所有组件并收尾。返回时即已关闭完成。 |




## Actor 接口


###  全局接口

| 函数/方法 | 说明 |
|-----------|------|
| `NewSystem(selfNodeID uint64, serializer lib.ISerializer) *System` | 创建单节点 Actor 系统 |
| `NewClusterSystem(selfNodeID uint64, serializer lib.ISerializer, transport cluster.ICluster) *ClusterSystem` | 创建集群版 Actor 系统 |
| `GetRouterForActor(actor iface.IActor) iface.IRouter` | 按 actor 类型获取（或创建并缓存）Router |
| `NewProcess(mailbox IMailbox) *Process` | 创建进程（内部使用） |
| `NewMailbox() *Mailbox` | 创建邮箱（内部使用） |
| `NewDefaultDispatcher(throughput int) IDispatcher` | 创建 goroutine 调度器 |
| `NewSynchronizedDispatcher(throughput int) IDispatcher` | 创建同步调度器 |
| `NewRouter() iface.IRouter` | 创建路由器


###  System

| 方法                                                                    | 说明 |
|-----------------------------------------------------------------------|------|
| `(*System)NodeId() uint64`                                                  | 本节点 ID |
| `(*System)NextID() uint64`                                                     | 生成下一个 ActorId |
| `(*System)Serializer() lib.ISerializer`                                        | 序列化器 |
| `(*System)SessionFactory() iface.ISessionFactory` / `SetSessionFactory(f)`     | Session 工厂（可选，gate 注入） |
| `(*System)Spawn(actor iface.IActor, args ...interface{}) *Pid`                 | 创建并注册新进程，投递 OnInit 后返回 Pid |
| `(*System)Register(ctx IContext) error`                                        | 将 Context 注册到 IdDict |
| `(*System)Unregister(ctx IContext) error`                                      | 从 IdDict 删除并 Unname |
| `(*System)Named(ctx IContext) error` / `Unname(ctx IContext) error`            | 注册/注销进程名字 |
| `(*System)SubmitTask(pid *Pid, task Task) error`                               | 异步投递任务 |
| `(*System)SubmitTaskAndWait(pid *Pid, task Task, timeout time.Duration) error` | 投递任务并等待完成 |
| `(*System)Send(message *ActorMessage) error`                                   | 异步发送消息（仅本节点） |
| `(*System)Call(message *ActorMessage) (data []byte, err error)`                | 同步发送并等待响应 |
| `(*System)GetProcess(ref interface{}) IProcess`                                | ref 可为 string \| uint64 \| *Pid |
| `(*System)GetAllProcesses() []IProcess`                                        | 所有已注册进程 |
| `(*System)ShutdownProcess(pid *Pid) error`                                     | 关闭指定进程 |
| `(*System)Shutdown() error`                                                    | 关闭系统（不等待进程完全退出） ||


###  ClusterSystem
在 System 基础上重写：

| 方法                                                     | 说明 |
|--------------------------------------------------------|------|
| `(*ClusterSystem)Send(message *ActorMessage) error`    | 本节点走 System.Send，跨节点走 transport.Send |
| `(*ClusterSystemCall(message *ActorMessage) (data []byte, err error)` | 本节点走 System.Call，跨节点走 transport.Call 并反序列化 Response |





## cluster 接口

`pkg/cluster` 提供集群通信与成员管理，对外以 **ICluster** 接口呈现：封装服务发现与消息队列，提供 Run、订阅/收发、注册与查询、选路、Watch、Shutdown。默认实现为 **Cluster**，内部依赖 pkg/discovery 与 pkg/messageQue，此处仅说明 ICluster 接口。


## 2. ICluster 接口

| 方法                                                                                              | 说明 |
|-------------------------------------------------------------------------------------------------|------|
| `(*Cluster)Run(ctx context.Context) error`                                                      | 启动内部消息队列与服务发现 |
| `(*Cluster)Subscribe(nodeId uint64, subscriber messageQue.ISubscriber) (messageQue.ISubscription, error)` | 以 nodeId 为 subject 订阅，收到消息时回调 subscriber.OnMessage |
| `(*Cluster)Send(nodeId uint64, message interface{}) error`                                                | 将 message 序列化后发送到 nodeId 对应 subject（单向） |
| `(*Cluster)Call(nodeId uint64, message interface{}, timeout time.Duration) ([]byte, error)`               | 序列化后请求 nodeId，阻塞至收到回复或超时，返回反序列化前的 data |
| `(*Cluster)Register(member *discovery.Member) error`                                                      | 注册成员到服务发现 |
| `(*Cluster)Deregister(memberId uint64) error`                                                             | 从服务发现注销成员 |
| `(*Cluster)Update(member *discovery.Member) error`                                                        | 更新成员信息 |
| `(*Cluster)Select(tag string, strategy RouteStrategy) (uint64, error)`                                    | 按 tag 取成员列表，用 strategy 选一个成员，返回其 GetID()；无成员或选路失败返回 ErrNotFoundMember |
| `(*Cluster)GetById(memberId uint64) *discovery.Member`                                                    | 按 ID 查询成员 |
| `(*Cluster)GetByKind(kind string) map[uint64]*discovery.Member`                                           | 按 Kind 查询成员映射 |
| `(*Cluster)GetByTag(tag string) []*discovery.Member`                                                      | 按 Tag 查询成员列表 |
| `(*Cluster)GetAll() map[uint64]*discovery.Member`                                                         | 返回所有成员 |
| `(*Cluster)Watch(kind string, handler discovery.ServiceChangeHandler)`                                    | 注册拓扑变更回调 |
| `(*Cluster)Unwatch(kind string, handler discovery.ServiceChangeHandler)`                                  | 取消拓扑变更回调 |
| `(*Cluster)Shutdown(ctx context.Context) error`                                                           | 关闭（先服务发现再消息队列） |






## profile 接口
配置文件格式为 YAML（或 viper 支持的其他格式）

| 方法                                                 | 说明 |
|----------------------------------------------------|------|
| `(*Profile)Get(key string, cfg interface{}) error` | 将配置中 key 对应内容反序列化到 cfg |
| `(*Profile)GetCluster() *cluster.Config`                     | 读取 `cluster` 配置并填充默认值 |
| `(*Profile)GetLogger() *glog.Config`                         | 读取 `logger` 配置并填充默认值 |
| `(*Profile)IsSingleNodeMode() bool`                          | 读取顶层布尔配置 `single-node` |





## network 接口  
提供统一网络层抽象与多协议实现 支持 tcp://、udp://、ws://、wss://

###  IServer
| 方法                 | 说明 |
|--------------------|------|
| `NewServer(handler IHandler, protoAddr string, option ...Option) (IServer, error)` | 根据 protoAddr 解析协议并创建对应 Server；handler 不可 nil |
| `(*IServer)Start() error` | 启动监听 |
| `(*IServer)Addr() string`    | 监听地址 |
| `(*IServer)Shutdown(ctx)`    | 优雅关闭 |


###  IConnection

| 方法                                                                          | 说明 |
|-----------------------------------------------------------------------------|------|
| `(*IConnection)ID() int64`                                                             | 连接唯一 ID |
| `(*IConnection)Send(msg []byte) error`                                                    | 发送 |
| `(*IConnection)LocalAddr() / RemoteAddr() string`                                         | 地址 |
| `(*IConnection)IsStop() bool`                                                             | 是否已关闭 |
| `(*IConnection)Type() ConnType`                                                           | Accept / Connect |
| `(*IConnection)Context() / SetContext(interface{})`                                       | 用户数据（如 Gate 存 Pid） |
| `(*IConnection)SetReadBuffer / SetWriteBuffer / SetLinger / SetNoDelay / SetTCPKeepAlive` | 选项 |
| `(*IConnection)Close(err error) error`                                                    | 关闭，可重复调用 |
###  IHandler

| 方法 | 说明 |
|------|------|
| `OnConnect(conn IConnection) error` | 连接建立；返回错误则关闭连接 |
| `OnMessage(conn IConnection, data []byte) (int, error)` | 收到数据；返回已处理字节数与错误 |
| `OnClose(conn IConnection, err error)` | 连接关闭 |


## Log
使用zap实现全局日志系统

| 函数 | 说明                                                                          |
|------|-----------------------------------------------------------------------------|
| `Init(cfg *Config)` | 初始化全局 logger：根据 cfg 设置 AtomicLevel、EncoderConfig、lumberjack 文件 Core， |
| `Stop() error` | 对当前 logger 与 sugared 分别 Sync，返回最后一次 Sync 的错误（若有）。进程退出前调用以刷盘。 |
| `SetLogLevel(level zapcore.Level)` | 设置全局日志级别（原子生效）。 |
| `GetLevel() zapcore.Level` | 返回当前全局日志级别。 |
| `WithOptions(opts ...zap.Option)` | 在现有 logger 上应用 zap.Option（如 AddCallerSkip、Hooks），将新 logger 及其 Sugar 存回包级 atomic.Value，后续调用使用新 logger。 |
| `Debugf(template string, args ...interface{})` | 输出 Debug 级别。                                                                |
| `Infof(template string, args ...interface{})` | 输出 Info 级别。                                                                 |
| `Warnf(template string, args ...interface{})` | 输出 Warn 级别。                                                                 |
| `Errorf(template string, args ...interface{})` | 输出 Error 级别。                                                                |
| `DPanicf(template string, args ...interface{})` | 输出 DPanic 级别（开发模式下降级为 Error）。                                               |
| `Panicf(template string, args ...interface{})` | 输出 Panic 级别并触发 panic；        |
| `Fatalf(template string, args ...interface{})` | 输出 Fatal 级别并退出； |
| `Debug(msg string, fields ...zap.Field)` | 输出 Debug 级别，不改变程序流程。                                                        |
| `Info(msg string, fields ...zap.Field)` | 结构化输出输出 Info 级别。                                                                 |
| `Warn(msg string, fields ...zap.Field)` | 结构化输出输出 Warn 级别。                                                                 |
| `Error(msg string, fields ...zap.Field)` | 结构化输出输出 Error 级别。                                                                |
| `Panic(msg string, fields ...zap.Field)` | 结构化输出输出 Panic 级别并触发 panic                     |
| `Fatal(msg string, fields ...zap.Field)` | 结构化输出输出 Fatal 级别并调用 os.Exit(1)；                    |



## Gate
Gate 为网关核心，负责监听地址、管理连接数、在连接建立时为每条连接创建 Agent 并投递解码后的消息。实现 `gateiface.IGate`。

### Gate 接口

| 方法                                              | 说明 |
|-------------------------------------------------|------|
| `(*Gate)Start(ctx context.Context) error`       | 启动网关；需先调用 SetSystem、SetAgentHandlerFactory、SetAddress。内部创建 network server、注册 Session 工厂并启动监听。 |
| `(*Gate)Stop(ctx context.Context) error`               | 关闭网络服务。 |
| `(*Gate)SetAddress(address string)`                    | 设置监听地址（如 tcp://127.0.0.1:9000）。 |
| `(*Gate)SetSystem(system iface.ISystem)`               | 设置 Actor 系统，Start 前必须调用。 |
| `(*Gate)SetAgentHandlerFactory(f AgentHandlerFactory)` | 设置每连接对应的业务 Handler 工厂（返回 `gateiface.IAgentHandler`），Start 前必须调用。 |
| `(*Gate)AppendOptions(options ...network.Option)`      | 设置网络选项（KeepAlive、缓冲区等）。 |
| `(*Gate)SetMaximumOfConn(n int64)`                     | 设置最大连接数，超过时新连接会被拒绝。 |
| `(*Gate)GetConnectionCount() int64`                    | 返回当前连接数。 |


###  IAgent 接口说明

 表示每条连接对应的一个 Agent,Agent继承Actor，Gate 在连接建立时通过 `AgentHandlerFactory()` 创建 Handler 并传给 `agent`。

| 类型/方法                                                           | 说明 |
|-----------------------------------------------------------------|------|
| `New(entity network.IConnection, handler IAgentHandler) *Agent` | 构造 Agent；handler 为 nil 时使用空实现。由 Gate 在 OnConnect 时调用并 Spawn。 |
| `(*IAgent)OnInit(agent IAgent) error`                                 | Agent 初始化完成后调用，可做业务初始化、挂中间件等。 |
| `(*IAgent)OnRoute(agent IAgent, data []byte) error`                      | 收到一条消息的 Body 时调用，data 为解码后的包体。 |
| `(*IAgent)OnStop(agent IAgent) error`                                    | Agent 即将停止时调用，可做清理。 |
| `(*IAgent)Context() iface.IContext`                                      | 返回当前 Actor 的 IContext。 |
| `(*IAgent)GetEntity() network.IConnection`                               | 返回绑定的连接。 |
| `(*IAgent)GetSession() *session.Session`                                 | 返回 Session。 |
| `(*IAgent)AppendMiddleware(...IMiddleware)`                              | 追加中间件。 |
| `(*IAgent)SetMiddleware(chain []IMiddleware)`                            | 替换整条中间件链。 |
| `(*IAgent)GetMiddleware() []IMiddleware`                                 | 返回当前中间件链。 |
| `(*IAgent)Push(msg *protocol.Message) error`                             | 经 RunBeforeEncode 后编码并发送到连接。 |
| `(*IAgent)SetValues(values map[string]string) error`                     | 合并 values 到 Session.Values。 |
| `(*IAgent)Shutdown() error`                                              | 关闭连接并关闭当前 Actor。 |


### SessionFactory
Session 封装会话数据（嵌 *pb.Session）以及向对端写数据的能力（Response、Push、Close、SyncValues 等）；写操作通过内部 transport 转为对 Agent 的 Actor 调用，最终在持有连接的 Agent 侧执行。

| 类型/方法 | 说明 |
|-----------|------|
| `Factory` | 实现 iface.ISessionFactory。 |
| `(m *Factory) FromRaw(ctx, raw *pb.Session) iface.ISession` | 用 raw 与 ctx 构造 Session，等价于 `New(raw, ctx)`。 |

### Session 接口
连接上下文 

| 类型/方法                                             | 说明 |
|---------------------------------------------------|------|
| `(*Session)SetString(key, value string)`                    | 设置 Values 中的字符串（不同步到对端）。 |
| `(*Session)GetString(key string) string`                    | 从 Values 取字符串。 |
| `(*Session)SetUint64(key, value)` / `GetUint64(key) uint64` | 以字符串形式存/取 uint64，Get 解析失败返回 0。 |
| `(*Session)SetInt64(key, value)` / `GetInt64(key) int64`    | 以字符串形式存/取 int64，Get 解析失败返回 0。 |
| `(*Session)SyncValues() error`                              | 将当前 Values 同步到对端。 |
| `(*Session)Response(data []byte) error`                     | 向对端推送业务响应（沿用当前请求的 Index/Cmd/Act/Tag）。 |
| `(*Session)ResponseErr(errCode uint16) error`               | 推送仅带错误码的无 body 消息。 |
| `(*Session)Push(cmd, act uint8, data []byte) error`         | 按 cmd/act 向对端推送带 body 消息。 |
| `(*Session)Close() error`                                   | 通知对端关闭连接。 |
| `(*Session)SetMessage(msg *protocol.Message)`               | 设置当前请求消息并写入 Values（base64），供集群序列化后 GetMessage 使用。 |
| `(*Session)GetMessage() *protocol.Message`                  | 返回当前请求消息（必要时从 Values 解码）。 |
| `(*Session)GetAgent() *iface.Pid`                           | 返回 Session 绑定的 Agent Pid。 |
| `(*Session)Raw() *pb.Session`                            | 返回 *pb.Session（会先把当前 msg 写回 Values）。 |


### Message

| 类型/函数                                                                                          | 说明 |
|------------------------------------------------------------------------------------------------|------|
| `New(cmd, act uint8, data []byte) *Message`                                                    | 构造消息，Len 由 codec 按 Data 长度写入。 |
| `NewData(data []byte) *Message`                                                                | 构造 (0,0) 纯数据消息。 |
| `NewErr(err uint16) *Message`                                                                  | 构造仅带错误码的消息。 |
| `NewDecoded(bodyLen, cmd, act, errCode, index, tag uint8/uint32/uint16, data []byte) *Message` | codec 解码时构造完整消息，包外一般不需直接调用。 |
| `(*Message)Copy(old *Message)`                                                                 | 从 old 复制 Cmd、Act、Index、Tag，用于回包。 |
| `(*Message)ID() uint16`                                                                                  | 返回 Cmd<<8+Act。 |
| `(*Message)GetLen()/SetLen(v)`                                                                           | 包体长度。 |
| `(*Message)GetCmd()/SetCmd(v)`                                                                           | 命令。 |
| `(*Message)GetAct()/SetAct(v)`                                                                           | 动作。 |
| `(*Message)GetError()/SetError(v)`                                                                       | 错误码。 |
| `(*Message)GetIndex()/SetIndex(v)`                                                                       | 序号。 |
| `(*Message)GetTag()/SetTag(v)`                                                                           | 标签（业务或中间件使用）。 |
| `(*Message)Clone() *Head`                                                                                | 返回 Head 副本。 |

工具函数

| 函数 | 说明 |
|------|------|
| `CmdAct(cmd, act uint8) uint16` | 将 cmd、act 合并为 16 位 ID。 |
| `ParseId(msgId uint16) (cmd, act uint8)` | 将 msgId 拆成 cmd、act。 |