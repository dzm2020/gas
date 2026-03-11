# internal/pb 模块文档

## 1. 模块功能概述

`internal/pb` 为 protobuf 生成代码包，由 `tools/proto/actor.proto` 经 `protoc` 生成（如 `actor.pb.go`）。

- **Pid**：进程标识，含 NodeId、ActorId、ActorName，用于跨节点寻址与命名。
- **Message**：To、From、Method、Data、Session、Async、Deadline 等，用于 Actor Send/Call 与集群传输。
- **Session**：Id、Agent（Pid）、Values（map）等，随消息传递，供 gate/session 与 actor 路由使用。
- **Response**：Data、ErrMsg，Call 的响应体。

本包不包含业务逻辑，仅类型定义与序列化；接口与用法见 `internal/iface` 与 `internal/actor`。

## 2. 接口文档

使用方式以生成的 Go 类型为准（Get/Set 方法、proto.Message 实现）。若需扩展，应修改 proto 后重新生成，或通过 iface 中的包装类型（如 ActorMessage、Response）扩展行为。

## 3. 设计结构

- 无协程；struct 为纯数据结构。
- 依赖：`google.golang.org/protobuf`。生成脚本见 `tools/proto/gen.bat`。
