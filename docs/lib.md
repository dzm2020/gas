# pkg/lib 模块文档

---

## 0. 本仓库模块边界与 `go get`

本仓库 **根模块**（`github.com/dzm2020/gas`）按 Go 惯例将大量实现放在 **`internal/`** 下：根据 Go 规则，**其他模块无法 `import` 本仓库的 `internal/...`**。

因此：

- README 与 **`examples/`** 中的 `import "github.com/dzm2020/gas/internal/..."` **仅在本仓库根目录作为 `main` 或测试构建时有效**；若你在**另一个** Go 模块里执行 `go get github.com/dzm2020/gas`，不能直接复制这些 import 路径编译通过。
- 当前定位可理解为：**单模块应用 / 以本仓库为根的框架**，对外可复用部分主要在 **`pkg/`**（如 `pkg/cluster`、`pkg/network`、`pkg/discovery` 等）；若需第三方与示例一致的 Node/Gate/Actor 组装方式，需将相应 API **逐步下沉到 `pkg/`**（中长期演进），或采用 **fork / 模块 replace** 等同源复用方式。

---

## 1. 包的概述

`pkg/lib` 由多个**子包**组成，无根包；各子包独立导入使用。

- **serializer**：序列化接口与 Json/MsgPack/PB 实现，供集群、Actor 等序列化消息。
- **timer**：基于 timingwheel 的 AfterFunc、DeadlineToTimeout，供 Actor 定时任务与超时计算。
- **waiter**：带超时的 ChanWaiter，供 Actor Call 同步等待响应。
- **mpsc**：无锁无界 MPSC 队列，供 Actor Mailbox 投递消息。
- **uid**：雪花式 ID 生成（Init、NextId、ParseId）。
- **strutil**：字符串工具（如首字母是否大写）。
- **stopper**：原子停止标记，供组件/服务优雅关闭。
- **component**：组件生命周期接口 IComponent[T]、IManager[T]，以及 Manager[T] 实现（按注册顺序 Start、逆序 Stop）。
- **xerror**：错误包装（Wrap/Wrapf）、Assert、PrintCoreDump。
- **fileutil**：按路径加载 Json/Yaml/配置文件。
- **buffer**：可读写缓冲区，支持扩容与 io.Reader/Writer。
- **event**：泛型事件监听器 Listener[V]，Register/UnRegister/Notify。
- **factory**：按名称注册/获取构造函数，泛型 Manager[T]。
- **grs**：全局 goroutine 管理（Go/GoTry、SetPanicHandler、Shutdown、WaitWithContext）。
- **netutil**：Socket 选项（SetReuseAddr、SetReusePort、SetTCPNoDelay、SetTCPKeepAlive、SetRcvBuffer、SetSndBuffer、SetTCPLinger 等），ListenConfig 封装。

---

## 2. 包的接口说明

### 2.1 serializer

| 类型/变量/接口 | 说明 |
|----------------|------|
| `ISerializer` | 接口：`Unmarshal(data []byte, msg interface{}) error`、`Marshal(msg interface{}) ([]byte, error)`。 |
| `Json` | 实现 ISerializer，基于 encoding/json。 |
| `MsgPack` | 实现 ISerializer，基于 msgpack。 |
| `PB` | 实现 ISerializer，基于 proto，msg 须为 proto.Message。 |

nil、[]byte 有前置处理；PB 非 proto.Message 时返回 ErrNotPBMsg。

---

### 2.2 timer

| 类型/函数 | 说明 |
|-----------|------|
| `Timer` | 结构体，嵌 *timingwheel.Timer。 |
| `AfterFunc(duration time.Duration, callback func()) *Timer` | 注册一次性定时器，到期在 timingwheel 中执行 callback。 |
| `DeadlineToTimeout(sec, nsec int64) time.Duration` | 将 Unix 时间戳 (sec, nsec) 转为相对当前的 timeout。 |

---

### 2.3 waiter

| 类型/函数/方法 | 说明 |
|----------------|------|
| `NewChanWaiter[T any](timeout time.Duration) *ChanWaiter[T]` | 创建带超时的 Waiter。 |
| `ChanWaiter[T]` | 结构体，持 dataChan、errChan、after。 |
| `(w *ChanWaiter[T]) Wait() (T, error)` | 阻塞直到 Done 或超时；超时返回 timeout 错误。 |
| `(w *ChanWaiter[T]) Done(rsp T, err error)` | 唤醒 Wait，传入结果或错误。 |

---

### 2.4 mpsc

| 类型/函数/方法 | 说明 |
|----------------|------|
| `NewMpsc() *Mpsc` | 创建无界 MPSC 队列。 |
| `Mpsc` | 结构体，多生产者单消费者。 |
| `(q *Mpsc) Push(x interface{})` | 入队。 |
| `(q *Mpsc) Pop() interface{}` | 出队，空返回 nil。 |
| `(q *Mpsc) Empty() bool` | 是否为空。 |

---

### 2.5 uid

| 类型/函数 | 说明 |
|-----------|------|
| `IdWorker` | 结构体（包内使用，需先 Init）。 |
| `Init(workerId int64)` | 初始化全局 IdWorker，workerId 需在合法范围内。 |
| `NextId() (int64, error)` | 生成下一个 ID（毫秒时间戳 + workerId + 序列号）；时钟回拨返回错误。 |
| `ParseId(id int64) (time.Time, int64, int64, int64)` | 解析 ID 为 (时间、时间戳 ms、workerId、序列号)。 |

包内常量含 CEpoch、CWorkerIdBits、CSSequenceBits、CWorkerIdShift、CTimeStampShift、CSequenceMask、CMaxWorker。

---

### 2.6 strutil

| 函数 | 说明 |
|------|------|
| `IsFirstLetterUppercase(s string) bool` | 判断 s 首字符是否为大写字母（空串 false）。 |

---

### 2.7 stopper

| 类型/方法 | 说明 |
|-----------|------|
| `Stopper` | 结构体，持 atomic.Bool。 |
| `(s *Stopper) IsStop() bool` | 是否已调用过 Stop。 |
| `(s *Stopper) Stop() bool` | CAS 置为已停止，仅第一次返回 true。 |

---

### 2.8 component

| 接口/类型 | 说明 |
|-----------|------|
| `IComponent[T]` | 接口：`Init(t T) error`、`Start(ctx, t T) error`、`Stop(ctx) error`、`Name() string`。 |
| `BaseComponent[T]` | 空实现 Init/Start/Stop，可嵌入。 |
| `IManager[T]` | 接口：Init、Start、Stop、Register、GetComponent、GetComponentNames、ComponentCount。 |
| `Manager[T]` | 结构体，实现 IManager[T]。 |
| `NewComponentsMgr[T any]() *Manager[T]` | 创建组件管理器。 |
| `(cm *Manager[T]) Register(component IComponent[T]) error` | 注册组件，启动后不可再注册。 |
| `(cm *Manager[T]) GetComponent(name string) IComponent[T]` | 按名称取组件。 |
| `(cm *Manager[T]) GetComponentNames() []string` | 已注册组件名称列表（按注册顺序）。 |
| `(cm *Manager[T]) ComponentCount() int` | 已注册数量。 |
| `(cm *Manager[T]) Init(t T) error` | 对全部组件调用 Init。 |
| `(cm *Manager[T]) Start(ctx, t T) error` | 按注册顺序 Start 所有组件；仅允许一次。 |
| `(cm *Manager[T]) Stop(ctx) error` | 按注册逆序 Stop 所有组件。 |
| `(cm *Manager[T]) IsStarted() bool` | 是否已 Start。 |
| `(cm *Manager[T]) IsStopped() bool` | 是否已 Stop。 |

Register/Start/Stop 在非法调用时返回包内错误变量（如 ErrCannotRegisterComponentAfterStarted、ErrComponentAlreadyRegistered 等）。

---

### 2.9 xerror

| 函数 | 说明 |
|------|------|
| `Wrap(err error, message string) error` | 包装 err，格式为 "message: err"；err 为 nil 返回 nil。 |
| `Wrapf(err error, format string, args ...interface{}) error` | 格式化 message 后包装 err；err 为 nil 返回 nil。 |
| `Assert(err error)` | err 非 nil 时 panic(err)。 |
| `PrintCoreDump()` | 供 defer 使用，recover 后把 panic 与 stack 写入当前目录时间戳-dump 文件。 |

---

### 2.10 fileutil

| 函数 | 说明 |
|------|------|
| `LoadJsonFile(path string, value interface{}) error` | 读取 path 并 json.Unmarshal 到 value。 |
| `LoadYamlFile(path string, value interface{}) error` | 读取 path 并 yaml.Unmarshal 到 value。 |
| `LoadConfigFile(path string, value interface{}) error` | 按扩展名选择 json 或 yaml 加载。 |

---

### 2.11 buffer

| 接口/类型/函数/方法 | 说明 |
|---------------------|------|
| `IBuffer` | 接口：Len、Cap、Available、Reset、Bytes、Readable、Write、WriteReader、WriteByte、Read、ReadByte、Peek、Skip、String。 |
| `Buffer` | 结构体，实现 IBuffer。 |
| `New(initialCap int) *Buffer` | 创建缓冲区，initialCap≤0 时默认 4096。 |
| `(b *Buffer) Len() int` | 可读字节数。 |
| `(b *Buffer) Cap() int` | 容量。 |
| `(b *Buffer) Available() int` | 剩余可写空间。 |
| `(b *Buffer) Reset()` | 清空，读写指针归零。 |
| `(b *Buffer) Bytes() []byte` | 当前可读切片（不复制）。 |
| `(b *Buffer) Readable() []byte` | 可读数据副本。 |
| `(b *Buffer) Write(data []byte) (int, error)` | 写入。 |
| `(b *Buffer) WriteReader(reader io.Reader) (int, error)` | 从 Reader 读入缓冲区。 |
| `(b *Buffer) WriteByte(c byte) error` | 写单字节。 |
| `(b *Buffer) Read(p []byte) (int, error)` | 读到 p。 |
| `(b *Buffer) ReadByte() (byte, error)` | 读单字节。 |
| `(b *Buffer) Peek(n int) ([]byte, error)` | 查看前 n 字节不移动读指针。 |
| `(b *Buffer) Skip(n int) error` | 读指针后移 n。 |
| `(b *Buffer) String() string` | 可读部分转字符串。 |

---

### 2.12 event

与 **Actor 事件总线**（`internal/actor`、`IContext.Subscribe`、`PublishLocal` / `PublishCluster`）不是同一套机制；对比说明见 **`docs/event.md` 第 7 节**。

| 类型/函数/方法 | 说明 |
|----------------|------|
| `Listener[V any]` | 结构体，泛型事件监听器。 |
| `NewListener[V any]() *Listener[V]` | 创建监听器。 |
| `(m *Listener[V]) Register(handler func(V))` | 注册处理函数；已存在同一函数指针则不再追加。 |
| `(m *Listener[V]) UnRegister(handler func(V))` | 注销处理函数。 |
| `(m *Listener[V]) Notify(param V)` | 同步调用所有已注册 handler(param)。 |

**实现说明**：`Register` / `UnRegister` 通过 **函数指针（reflect 比较）** 判断是否「同一 handler」。每次传入的**闭包**即使逻辑相同，也会被视为不同 handler，可能导致重复注册；热路径若需稳定注销，请使用**具名函数**或包级变量函数，而非内联 `func(...) { ... }` 闭包。

---

### 2.13 factory

| 类型/函数/方法 | 说明 |
|----------------|------|
| `New[T any]() *Manager[T]` | 创建工厂管理器。 |
| `Manager[T]` | 结构体，name -> 构造函数。 |
| `(f *Manager[T]) Register(name string, factory func(args ...any) (T, error)) error` | 注册工厂，同名返回错误。 |
| `(f *Manager[T]) Unregister(name string)` | 注销。 |
| `(f *Manager[T]) Get(name string) (func(args ...any) (T, error), bool)` | 获取工厂函数。 |
| `(f *Manager[T]) List() []string` | 已注册名称列表。 |

---

### 2.14 grs

| 函数 | 说明 |
|------|------|
| `Go(f func(ctx context.Context))` | 使用全局 panicHandler 启动 goroutine，传入 ctx。 |
| `GoTry(f func(ctx context.Context), try func(any))` | 启动 goroutine，panic 时调用 try。 |
| `Try(f func(), reFun func(err any))` | 同步执行 f，recover 后调用 reFun。 |
| `SetPanicHandler(handler func(interface{}))` | 设置全局 panic 回调。 |
| `Shutdown(ctx context.Context) error` | 取消全局 ctx、等待已 Add 的 WaitGroup；超时返回错误。 |
| `WaitWithContext(ctx context.Context, group *sync.WaitGroup)` | 在 goroutine 中 group.Wait()，ctx 取消或 Wait 完成后返回。 |

---

### 2.15 netutil

| 类型/函数 | 说明 |
|-----------|------|
| `ListenConfig` | 结构体，封装 net.ListenConfig 与 socket 选项。 |
| `NewListenConfig() *ListenConfig` | 创建 ListenConfig。 |
| `SetReuseAddr(conn net.Conn) error` | 设置 SO_REUSEADDR。 |
| `SetReusePort(conn net.Conn) error` | 设置 SO_REUSEPORT（部分系统支持）。 |
| `SetTCPNoDelay(conn net.Conn, enable bool) error` | 设置 TCP_NODELAY。 |
| `SetTCPKeepAlive(conn net.Conn, enable bool, period time.Duration) error` | 设置 KeepAlive。 |
| `SetRcvBuffer(conn net.Conn, rcvBuf int) error` | 设置接收缓冲区大小。 |
| `SetSndBuffer(conn net.Conn, sndBuf int) error` | 设置发送缓冲区大小。 |
| `SetTCPLinger(conn net.Conn, enable bool, lingerSec int) error` | 设置 SO_LINGER。 |
