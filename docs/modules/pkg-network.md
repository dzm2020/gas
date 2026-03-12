# pkg/network 模块文档

## 1. 模块功能概述

`pkg/network` 提供统一网络层抽象与多协议实现：

- **协议**：根据地址字符串解析协议（tcp、udp、ws/wss），创建对应 Server；支持 tcp://、udp://、ws://、wss://。
- **IHandler**：OnConnect、OnMessage(conn, data)、(int, error)、OnClose；业务实现，不包含编解码（编解码在 Gate 等上层）。
- **IConnection**：ID、Send、Close、LocalAddr、RemoteAddr、IsStop、Type、Context/SetContext、SetReadBuffer/SetWriteBuffer、SetLinger、SetNoDelay、SetTCPKeepAlive 等。
- **IServer**：Start、Shutdown(ctx)、Addr。
- **EmptyHandler**：空实现，可嵌入后只重写需要的方法。
- 内部有 base_server、base_connection 及 tcp/udp/websocket 具体实现，连接 ID 原子递增生成。

## 2. 接口文档

### 2.1 IHandler

| 方法 | 说明 |
|------|------|
| `OnConnect(conn IConnection) error` | 连接建立；返回错误则关闭连接 |
| `OnMessage(conn IConnection, data []byte) (int, error)` | 收到数据；返回已处理字节数与错误 |
| `OnClose(conn IConnection, err error)` | 连接关闭 |

### 2.2 IConnection

| 方法 | 说明 |
|------|------|
| `ID() int64` | 连接唯一 ID |
| `Send(msg []byte) error` | 发送 |
| `Close(err error) error` | 关闭，可重复调用 |
| `LocalAddr() / RemoteAddr() string` | 地址 |
| `IsStop() bool` | 是否已关闭 |
| `Type() ConnType` | Accept / Connect |
| `Context() / SetContext(interface{})` | 用户数据（如 Gate 存 Pid） |
| `SetReadBuffer / SetWriteBuffer / SetLinger / SetNoDelay / SetTCPKeepAlive` | 选项 |

### 2.3 IServer

| 方法 | 说明 |
|------|------|
| `Start() error` | 启动监听 |
| `Shutdown(ctx)` | 优雅关闭 |
| `Addr() string` | 监听地址 |

### 2.4 创建与常量

| 函数/常量 | 说明 |
|-----------|------|
| `NewServer(handler IHandler, protoAddr string, option ...Option) (IServer, error)` | 根据 protoAddr 解析协议并创建对应 Server；handler 不可 nil |
| `ConnType` | Accept / Connect |
| `EmptyHandler` | 空 Handler 实现 |

### 2.5 Option

通过 WithXxx 配置（如缓冲区、KeepAlive 等），见 options.go。

## 3. 设计结构

### 3.1 协程模型

- **Listen**：主 goroutine 或 Start 内 goroutine Accept。
- **每连接**：通常每连接一个 read goroutine（或 gnet 等事件循环），读到数据后调用 Handler.OnMessage；Send 可能进写缓冲区或写队列，由内部串行写。
- **Shutdown**：关闭 listener 并优雅关闭各连接，ctx 控制超时。

### 3.2 Struct 关系

```
NewServer(handler, protoAddr, options...)
  -> parseProtoAddr -> "tcp"|"udp"|"ws"|"wss"
  -> newBaseServer(network, address, handler, option...)
  -> NewTCPServer / NewUDPServer / NewWebSocketServer(base)

BaseServer / TCPServer / UDPServer / WebSocketServer
  -> 持有 listener、options、连接管理、handler
Connection: 实现 IConnection，持有 conn、id、context 等
```

### 3.3 依赖

- 标准库 net、context、time、sync/atomic
- gorilla/websocket
