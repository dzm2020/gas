# GAS 配置文件说明

本文档描述 GAS 框架所使用的配置文件结构。配置通过 **Profile** 组件加载，支持 Viper 所支持的格式（如 YAML、JSON 等），由 `profile.New(配置文件路径)` 指定路径后，在节点启动时读取并反序列化到各模块。

---

## 1. 配置文件整体结构

配置为键值结构，顶层键与用途如下：

| 顶层键 | 说明 | 必填 |
|--------|------|------|
| `node` | 当前节点信息（ID、类型、地址、端口等） | 是 |
| `single-node` | 是否单节点模式（不启用集群） | 否，默认 false |
| `cluster` | 集群配置（服务发现、消息队列） | 集群模式下必填 |
| `logger` | 日志配置 | 否，有默认值 |
| `gate` | 网关组件配置（启用 Gate 组件时使用） | 否，有默认值 |
| `redis` | Redis 组件配置（启用 Redis 组件时使用），为**数组** | 否 |

---

## 2. node（节点信息）

对应 `discovery.Member`，用于标识本节点并参与服务发现。

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uint64 | 节点 ID |
| `kind` | string | 节点类型 |
| `address` | string | 节点地址 |
| `port` | int | 节点端口 |
| `tags` | []string | 节点标签（可选） |
| `meta` | map[string]string | 节点元数据（可选） |
| `status` | string | 健康状态：`passing` / `warning` / `critical`，可选 |

**示例：**

```yaml
node:
  id: 1
  kind: game
  address: 127.0.0.1
  port: 9000
  tags: []
  meta: {}
```

---

## 3. single-node（单节点模式）

- **类型：** 布尔
- **说明：** 为 `true` 时表示单节点模式，不启用集群（服务发现、消息队列等）。
- **默认：** 未配置时按 false 处理。

```yaml
single-node: false
```

---

## 4. cluster（集群配置）

包含集群名称、服务发现与消息队列类型及对应参数。

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 集群名称，可为空 |
| `discovery` | object | 服务发现配置 |
| `messageQueue` | object | 消息队列配置 |

### 4.1 discovery（服务发现）

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 实现类型，当前支持：`consul` |
| `config` | object | 具体实现所需参数，见下表 |

**type = "consul" 时，config 字段：**

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `address` | string | `127.0.0.1:8500` | Consul 地址 |
| `watchWaitTime` | duration | 1s | Watch 等待时间 |
| `healthTTL` | duration | 1s | 健康检查 TTL |
| `deregisterInterval` | duration | 3s | 注销间隔 |

**示例：**

```yaml
cluster:
  name: my-cluster
  discovery:
    type: consul
    config:
      address: 127.0.0.1:8500
      watchWaitTime: 1s
      healthTTL: 1s
      deregisterInterval: 3s
  messageQueue:
    type: nats
    config:
      servers:
        - nats://127.0.0.1:4222
      name: gas-nats-client
      maxReconnects: -1
      reconnectWait: 1s
      timeout: 1s
      pingInterval: 1s
      maxPingsOut: 2
      allowReconnect: true
      retryOnFailedConnect: false
      poolSize: 10
```

### 4.2 messageQueue（消息队列）

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 实现类型，当前支持：`nats` |
| `config` | object | 具体实现所需参数，见下表 |

**type = "nats" 时，config 字段：**

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `servers` | []string | `["nats://127.0.0.1:4222"]` | NATS 服务器地址列表 |
| `name` | string | `gas-nats-client` | 客户端名称 |
| `maxReconnects` | int | -1 | 最大重连次数，-1 表示无限 |
| `reconnectWait` | duration | 1s | 重连等待时间 |
| `timeout` | duration | 1s | 连接超时 |
| `pingInterval` | duration | 1s | Ping 间隔 |
| `maxPingsOut` | int | 2 | 最大未响应 Ping 数 |
| `allowReconnect` | bool | true | 是否允许重连 |
| `username` | string | - | 用户名（可选） |
| `password` | string | - | 密码（可选） |
| `token` | string | - | Token 认证（可选） |
| `disableNoEcho` | bool | false | 是否禁用 NoEcho |
| `retryOnFailedConnect` | bool | false | 连接失败是否重试 |
| `poolSize` | int | 10 | 连接池大小 |

---

## 5. logger（日志配置）

对应 `glog.Config`。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `path` | string | `./logs/app.log` | 日志文件路径 |
| `level` | string | `info` | 级别：debug / info / warn / error / dpanic / panic / fatal |
| `printConsole` | bool | true | 是否同时输出到控制台 |
| `maxSize` | int | 500 | 单文件最大大小（MB），超过则切割 |
| `maxBackups` | int | 100 | 最大保留文件数 |
| `maxAge` | int | 30 | 保留天数 |
| `compress` | bool | false | 是否压缩旧日志 |
| `localTime` | bool | true | 是否使用本地时间 |

**示例：**

```yaml
logger:
  path: ./logs/app.log
  level: info
  printConsole: true
  maxSize: 500
  maxBackups: 100
  maxAge: 30
  compress: false
  localTime: true
```

---

## 6. gate（网关组件配置）

启用 Gate 组件时，从配置键 `gate` 读取，对应 `gate.Config`。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `address` | string | `tcp://127.0.0.1:9000` | 监听地址，格式：`tcp://host:port` 或 `udp://host:port` |
| `keepAlive` | int | 5 | 连接超时（秒），0 表示不检测 |
| `sendChanSize` | int | 1024 | 发送队列缓冲大小 |
| `readBufSize` | int | 4096 | 读缓冲区大小 |
| `maxConn` | int | 10000 | 最大连接数 |
| `tlsCertFile` | string | - | TLS 证书路径（与 tlsKeyFile 同时配置才启用 TLS） |
| `tlsKeyFile` | string | - | TLS 私钥路径 |

**示例：**

```yaml
gate:
  address: tcp://0.0.0.0:9000
  keepAlive: 5
  sendChanSize: 1024
  readBufSize: 4096
  maxConn: 10000
  # tlsCertFile: /path/to/cert.pem
  # tlsKeyFile: /path/to/key.pem
```

---

## 7. redis（Redis 组件配置）

启用 Redis 组件时，从配置键 `redis` 读取，值为 **数组**，每个元素对应一个 Redis 实例（下标即实例 ID）。

| 字段 | 类型 | 说明 |
|------|------|------|
| `address` | []string | Redis 地址列表（支持集群/单机） |
| `password` | string | 密码，空表示无密码 |
| `pool_size` | int | 连接池大小 |

**示例（多实例）：**

```yaml
redis:
  - address:
      - 127.0.0.1:6379
    password: ""
    pool_size: 10
  - address:
      - 127.0.0.1:6380
    password: "secret"
    pool_size: 20
```

---

## 8. 完整配置示例（YAML）

```yaml
# 节点信息（必填）
node:
  id: 1
  kind: game
  address: 127.0.0.1
  port: 9000
  tags: []
  meta: {}

# 单节点模式：true 则不使用集群
single-node: false

# 集群配置（非单节点时使用）
cluster:
  name: my-cluster
  discovery:
    type: consul
    config:
      address: 127.0.0.1:8500
      watchWaitTime: 1s
      healthTTL: 1s
      deregisterInterval: 3s
  messageQueue:
    type: nats
    config:
      servers:
        - nats://127.0.0.1:4222
      name: gas-nats-client
      maxReconnects: -1
      reconnectWait: 1s
      timeout: 1s
      poolSize: 10

# 日志
logger:
  path: ./logs/app.log
  level: info
  printConsole: true
  maxSize: 500
  maxBackups: 100
  maxAge: 30
  compress: false
  localTime: true

# 网关（按需启用 Gate 组件时填写）
gate:
  address: tcp://0.0.0.0:9000
  keepAlive: 5
  sendChanSize: 1024
  readBufSize: 4096
  maxConn: 10000

# Redis（按需启用 Redis 组件时填写，数组，下标为实例 ID）
redis:
  - address: [ "127.0.0.1:6379" ]
    password: ""
    pool_size: 10
```

---

## 9. 安全与敏感配置

- **勿将密钥、证书私钥、Redis/NATS 密码、加密中间件密钥等写入仓库**。生产环境应使用环境变量、密钥管理服务或部署时注入的配置，示例 YAML 中的占位符仅作格式说明。
- **Gate TLS**：`tlsCertFile` / `tlsKeyFile` 指向的文件权限应受限；私钥不可提交到版本库。
- **Gate 中间件**（加密、限流等）：加密插件涉及密钥交换与 XOR，限流涉及配额；参数多在代码中构造（见 `internal/component/gate/middleware`），勿在公开仓库中硬编码生产密钥或过高配额导致被滥用。
- **集群**：Consul / NATS 的 `token`、`password` 等字段与 `redis.password` 同属敏感信息，按组织安全策略管理。

---

## 10. 使用方式说明

- 配置文件路径在创建 Profile 时传入：`profile.New("config.yaml")`，由节点在启动时加载。
- 未配置的顶层键会使用各模块的默认值（如 `logger`、`gate`、`cluster` 的默认实现）。
- 各组件通过 `node.Profile().Get("键名", &结构体)` 读取对应片段，键名与上表中的顶层键一致（如 `cluster`、`logger`、`gate`、`redis`）。
- duration 在 YAML 中可使用如 `1s`、`2m` 等 Go duration 格式；在 JSON 中为数字时一般按纳秒解析，具体以 Viper 行为为准。

以上字段与代码中 `pkg/glog`、`pkg/cluster`、`pkg/discovery`、`internal/component/gate`、`internal/component/db/redis` 等处的 Config 结构一致，若有增减以代码为准。Redis 客户端依赖为 **`github.com/redis/go-redis/v9`**。
