# GAS 项目结构建议

## 当前结构分析

当前项目是一个基于 Actor 模型的游戏服务器框架，包含以下核心功能：
- Actor 系统（进程管理、消息路由）
- 网络通信（TCP/UDP）
- 服务发现（Consul）
- 消息队列（NATS）
- 网关（Gate）
- 节点管理

## 推荐的项目结构

```
gas/
├── cmd/                          # 应用程序入口
│   ├── game-node/               # 游戏节点入口
│   │   └── main.go
│   ├── gate-node/               # 网关节点入口
│   │   └── main.go
│   └── tools/                   # 工具程序
│       └── ...
│
├── internal/                     # 内部代码（不对外暴露）
│   ├── actor/                   # Actor 模型实现
│   │   ├── component.go         # Actor 组件
│   │   ├── context.go           # Actor 上下文
│   │   ├── dispatch.go          # 消息分发
│   │   ├── errors.go            # 错误定义
│   │   ├── mailbox.go           # 消息邮箱
│   │   ├── manager.go           # 名字管理器
│   │   ├── process.go           # 进程实现
│   │   ├── router.go            # 消息路由
│   │   ├── system.go            # Actor 系统
│   │   └── waiter.go            # 等待器
│   │
│   ├── config/                  # 配置管理
│   │   └── config.go
│   │
│   ├── gate/                    # 网关模块
│   │   ├── agent.go             # 代理
│   │   ├── gate.go              # 网关主逻辑
│   │   ├── codec/               # 编解码器
│   │   │   └── codec.go
│   │   └── protocol/            # 协议定义
│   │       └── message.go
│   │
│   ├── iface/                   # 接口定义（核心抽象）
│   │   ├── actor.go             # Actor 相关接口
│   │   ├── actor.pb.go          # Protobuf 生成代码
│   │   ├── component.go         # 组件接口
│   │   ├── message.go           # 消息接口
│   │   ├── node.go              # 节点接口
│   │   ├── remote.go            # 远程通信接口
│   │   └── session.go           # 会话接口
│   │
│   ├── node/                    # 节点管理
│   │   ├── component.go         # 节点组件
│   │   └── node.go              # 节点实现
│   │
│   └── remote/                  # 远程通信
│       ├── component.go         # 远程组件
│       ├── remote.go            # 远程通信实现
│       └── routeStrategy.go     # 路由策略
│
├── pkg/                         # 可复用的公共包（可被外部使用）
│   ├── discovery/               # 服务发现
│   │   ├── config.go
│   │   ├── factory.go
│   │   ├── iface/               # 服务发现接口
│   │   │   ├── discovery.go
│   │   │   ├── list.go
│   │   │   └── node.go
│   │   └── provider/            # 服务发现提供者
│   │       └── consul/
│   │
│   ├── glog/                    # 日志库
│   │   ├── config.go
│   │   ├── logger.go
│   │   ├── options.go
│   │   └── writer.go
│   │
│   ├── lib/                     # 工具库
│   │   ├── buffer.go            # 缓冲区
│   │   ├── error.go             # 错误处理
│   │   ├── event.go             # 事件
│   │   ├── goroutine.go         # 协程管理
│   │   ├── mpsc.go              # 多生产者单消费者队列
│   │   ├── netx.go              # 网络工具
│   │   ├── reflect.go           # 反射工具
│   │   ├── serializer.go        # 序列化器
│   │   ├── time.go              # 时间工具
│   │   └── uid.go               # ID 生成器
│   │
│   ├── messageque/             # 消息队列
│   │   ├── config.go
│   │   ├── factory.go
│   │   ├── iface/               # 消息队列接口
│   │   │   └── iface.go
│   │   └── provider/            # 消息队列提供者
│   │       ├── nats/
│   │       └── network/
│   │
│   └── network/                 # 网络库
│       ├── base_connection.go   # 基础连接
│       ├── manager.go           # 连接管理器
│       ├── network.go           # 网络接口
│       ├── options.go           # 选项
│       ├── tcp_client.go        # TCP 客户端
│       ├── tcp_connection.go    # TCP 连接
│       ├── tcp_server.go        # TCP 服务器
│       ├── udp_connection.go     # UDP 连接
│       └── udp_server.go        # UDP 服务器
│
├── examples/                    # 示例代码
│   └── cluster/                 # 集群示例
│       ├── client/              # 客户端示例
│       ├── common/              # 公共代码
│       ├── conf/                # 配置文件
│       ├── game-node/           # 游戏节点示例
│       └── gate-node/           # 网关节点示例
│
├── api/                         # API 定义（可选）
│   └── proto/                   # Protobuf 定义
│       └── actor.proto
│
├── configs/                     # 配置文件模板（可选）
│   ├── game-node.example.json
│   └── gate-node.example.json
│
├── scripts/                     # 脚本文件（可选）
│   ├── build.sh
│   └── deploy.sh
│
├── docs/                        # 文档（可选）
│   ├── architecture.md          # 架构文档
│   ├── getting-started.md       # 快速开始
│   └── api.md                   # API 文档
│
├── tools/                       # 开发工具和第三方工具
│   ├── consul/                  # Consul 工具
│   ├── nats/                    # NATS 工具
│   └── proto/                   # Protobuf 工具
│
├── .gitignore                   # Git 忽略文件
├── go.mod                       # Go 模块定义
├── go.sum                       # Go 依赖校验
├── README.md                    # 项目说明
└── PROJECT_STRUCTURE.md        # 项目结构说明（本文件）
```

## 结构说明

### 1. cmd/ - 应用程序入口
- **目的**: 存放应用程序的 main 函数
- **原则**: 每个可执行程序一个目录
- **好处**: 清晰的入口点，便于构建和部署

### 2. internal/ - 内部代码
- **目的**: 存放项目内部实现，不对外暴露
- **原则**: 
  - `internal/iface/` - 核心接口定义，其他模块依赖它
  - `internal/actor/` - Actor 系统核心实现
  - `internal/node/` - 节点管理
  - `internal/gate/` - 网关模块
  - `internal/config/` - 配置管理
  - `internal/remote/` - 远程通信

### 3. pkg/ - 可复用包
- **目的**: 可被外部项目使用的公共包
- **原则**: 
  - 每个包应该有清晰的职责
  - 提供稳定的 API
  - 包含必要的文档和示例

### 4. examples/ - 示例代码
- **目的**: 提供使用示例
- **原则**: 示例应该完整、可运行

### 5. tools/ - 工具
- **目的**: 存放开发工具和第三方工具
- **注意**: 这些工具通常不参与编译

## 改进建议

### 1. 创建 cmd/ 目录
将示例中的 main 函数移到 `cmd/` 目录下，使项目结构更清晰。

### 2. 统一接口定义
- `internal/iface/` 应该只包含接口定义
- 实现应该放在对应的模块中（如 `internal/actor/`）

### 3. 配置管理
- 考虑将配置文件模板放在 `configs/` 目录
- 示例配置放在 `examples/` 中

### 4. 文档完善
- 在 `docs/` 目录下添加架构文档、API 文档等
- 每个包应该有清晰的包注释

### 5. 测试文件
- 单元测试文件应该与源文件放在同一目录
- 集成测试可以放在 `test/` 目录

### 6. 代码组织
- 每个包应该有一个 `doc.go` 文件说明包的用途
- 相关文件应该放在同一个目录下

## 文件命名规范

1. **接口文件**: `iface.go` 或 `interface.go`
2. **实现文件**: 使用描述性的名称，如 `actor.go`, `system.go`
3. **错误文件**: `errors.go`
4. **测试文件**: `*_test.go`
5. **文档文件**: `doc.go`

## 依赖关系

```
cmd/ (应用程序入口)
  └──> internal/ (内部实现)
        └──> pkg/ (公共包)
              └──> pkg/ (pkg 内部可以相互依赖)
```

### 依赖规则详解

1. **cmd/** 
   - 可以依赖 `internal/` 和 `pkg/`
   - 是应用程序的入口点

2. **internal/** 
   - 可以依赖 `pkg/`（公共包）
   - **不应该依赖其他 `internal/` 子包**（除了 `iface/`）
   - 原因：避免循环依赖，保持模块独立性
   - 例外：`internal/iface/` 是接口定义，其他模块可以依赖它

3. **pkg/** 
   - **可以依赖其他 `pkg/` 子包**（这是正常的！）
   - 例如：`pkg/network/` 可以使用 `pkg/glog/` 进行日志记录
   - 不应该依赖 `internal/`（保持公共包的独立性）
   - 原因：`pkg/` 是可复用的公共包，它们之间可以相互协作

### 实际示例

✅ **允许的依赖**：
- `pkg/network/` → `pkg/glog/` （网络包使用日志包）
- `pkg/discovery/` → `pkg/glog/` （服务发现使用日志包）
- `internal/actor/` → `pkg/lib/` （Actor 使用工具库）
- `internal/actor/` → `internal/iface/` （Actor 依赖接口定义）

❌ **不允许的依赖**：
- `pkg/network/` → `internal/actor/` （公共包不能依赖内部实现）
- `internal/actor/` → `internal/gate/` （内部子包不应相互依赖）
- `pkg/glog/` → `pkg/network/` （避免循环依赖，日志不应该依赖网络）

## 总结

当前项目结构已经比较合理，主要改进点：
1. 创建 `cmd/` 目录统一管理入口程序
2. 完善文档和注释
3. 统一配置管理
4. 加强包的职责划分

这样的结构既符合 Go 社区的最佳实践，也便于项目的维护和扩展。

