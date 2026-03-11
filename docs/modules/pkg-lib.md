# pkg/lib 模块文档

## 1. 模块功能概述

`pkg/lib` 提供通用工具子包：

- **serializer**：ISerializer（Marshal/Unmarshal）；实现 Json、MsgPack、PB（proto）；前置检查 nil、[]byte 透传。
- **stopper**：Stopper 结构体，atomic.Bool 标记已停，Stop() CAS 一次。
- **component**：IComponent[T]（Init、Start、Stop、Name）、BaseComponent[T] 空实现；IManager[T]（Init、Start、Stop、Register、GetComponent、GetComponentNames、ComponentCount）；Manager[T] 按注册顺序 Start、逆序 Stop，started/stopped 原子状态。
- **xerror**：错误包装与 PrintCoreDump 等。
- **mpsc**：无界 MPSC 队列，Push/Pop/Empty，供 actor.Mailbox 使用。
- **waiter**：ChanWaiter（如 Wait/Done），用于 Call 同步等待。
- **time**：AfterFunc、DeadlineToTimeout 等。
- **uid**、**string**、**file**、**buffer**、**event**、**factory**、**grs**、**netutil**：各子包工具。

## 2. 接口文档

### 2.1 lib（根）

| 类型/变量 | 说明 |
|-----------|------|
| `ISerializer` | Unmarshal(data, msg)、Marshal(msg) |
| `Json` / `MsgPack` / `PB` | 预定义实现 |
| `Mpsc` | Push、Pop、Empty（无界 MPSC） |
| `NewMpsc()` | 创建 Mpsc |
| `ChanWaiter[T]` | Done(value, err)、Wait()（带超时等，见 waiter） |
| `AfterFunc`、`Timer`、`DeadlineToTimeout` | 见 time 等 |

### 2.2 lib/stopper

| 类型 | 说明 |
|------|------|
| `Stopper` | IsStop() bool、Stop() bool（CAS 仅一次 true） |

### 2.3 lib/component

| 接口/类型 | 说明 |
|-----------|------|
| `IComponent[T]` | Init(t)、Start(ctx, t)、Stop(ctx)、Name() string |
| `BaseComponent[T]` | 空实现 Init/Start/Stop |
| `IManager[T]` | Init、Start、Stop、Register、GetComponent、GetComponentNames、ComponentCount |
| `Manager[T]` | NewComponentsMgr[T]()；Register 后按 order Start、逆序 Stop；Start/Stop 仅允许一次，Stop 用 sync.Once |

错误：ErrComponentCannotBeNil、ErrComponentNameCannotBeEmpty、ErrCannotRegisterComponentAfterStarted、ErrComponentAlreadyRegistered、ErrManagerAlreadyStarted、ErrManagerStoppedCannotRestart、ErrFailedToStartComponent。

### 2.4 lib/xerror

| 函数 | 说明 |
|------|------|
| `Wrap(err, msg)` / `Wrapf(err, format, args...)` | 包装错误 |
| `PrintCoreDump()` | defer 用，崩溃时打印等 |

### 2.5 其他子包（简要）

- **grs**：goroutine 池、PanicHandler、Shutdown 等。
- **buffer**：缓冲区复用。
- **event**：事件发布订阅。
- **factory**：类型名到构造函数的注册。
- **netutil**：SetLinger、SetNoDelay 等 socket 选项（Unix/Windows）。

## 3. 设计结构（协程模型与 struct 关系）

### 3.1 协程模型

- **Manager[T]**：Start/Stop 在调用方 goroutine；组件内部可自行起 goroutine。
- **Mpsc**：多生产者单消费者，actor.Mailbox 中单 goroutine 消费，Push 可能来自多 goroutine。
- **ChanWaiter**：Call 方 Wait 阻塞，目标 Actor 在 Mailbox 调度 goroutine 中 Response 唤醒。

### 3.2 Struct 关系

```
Manager[T]
  ├── components ConcurrentMap[string, IComponent[T]]
  ├── order []string, orderMu, started, stopped, stopOnce
  └── Start 按 order 调用 component.Start；Stop 逆序 component.Stop

Stopper: isStopped atomic.Bool
Mpsc: 内部队列（channel 或 slice+mutex 等实现）
```

### 3.3 依赖

- 标准库 sync、sync/atomic、context、time
- github.com/duke-git/lancet/v2、google.golang.org/protobuf、github.com/vmihailenco/msgpack/v5 等
