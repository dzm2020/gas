# redis 模块文档

---

## 1. 组件接口说明（Component）

Redis 组件将多组 Redis 连接挂到 Node 生命周期：Start 时从 profile 读取配置数组，按下标 Add 到全局管理器，并为所有 Client 预加载已注册脚本；Stop 时关闭全部连接。

| 类型/方法 | 说明 |
|-----------|------|
| `Component` | 结构体，嵌 `component.BaseComponent[iface.INode]`。 |
| `NewComponent() *Component` | 创建 Redis 组件实例。 |
| `Name() string` | 返回组件名（profile 配置键名）。 |
| `Start(ctx context.Context, node iface.INode) error` | 使用 profile.Get(Name(), &configs) 读取配置数组，对每项 Add(i, config)；再 Range 所有 Client 执行 loadAllScripts；任一失败返回错误。 |
| `Stop(ctx context.Context) error` | 调用 Close() 关闭所有已添加的 Client。 |

---

## 2. 多实例管理（Manager）

通过数字 id 管理多组 Redis 连接，内部使用 sync.Map 存储 id -> *Client；供组件或业务按 id 取用、遍历或关闭。

| 函数 | 说明 |
|------|------|
| `Get(id int) *Client` | 按 id 返回已添加的 Client，不存在返回 nil。 |
| `Add(id int, conf *Config) error` | 若 id 已存在则直接返回 nil；否则根据 conf 创建 Client（内部 Ping 校验），成功后存入并返回 nil。 |
| `Has(id int) bool` | 判断 id 是否已有对应 Client。 |
| `Range(fn func(*Client))` | 遍历当前所有 Client，对每个调用 fn；顺序不保证。 |
| `Close()` | 遍历所有 Client 执行 Close() 并从映射中删除。 |

---

## 3. Client 与 Config

### 3.1 Config

| 类型 | 说明 |
|------|------|
| `Config` | 配置结构体，含 json/yaml 标签。 |
| 字段 | `Address []string` 地址列表；`Password string` 密码；`PoolSize int` 连接池大小。 |

创建 Client 时通过 `Add(id, conf)` 传入 Config；内部使用 `redis.NewUniversalClient(UniversalOptions{Addrs, Password, PoolSize})`，支持单机与集群等模式。

### 3.2 Client

| 类型 | 说明 |
|------|------|
| `Client` | 结构体，嵌 `redis.UniversalClient`，并持有一份 `conf *Config`。 |

Client 不对外提供构造函数；由 `Add(id, conf)` 内部在 Ping 成功后创建并存入管理器。业务通过 `Get(id)` 取得 *Client 后，可直接使用 go-redis 的 UniversalClient 接口（Get/Set/Do 等）。

---

## 4. 辅助函数

| 函数 | 说明 |
|------|------|
| `IsNil(err error) bool` | 判断 err 是否为 Redis 的 Nil（如键不存在），即 errors.Is(err, redis.Nil)。 |
| `Error(err error) bool` | 判断是否为“真正”错误：若为 redis.Nil 返回 false，否则返回 err != nil。 |

用于在业务中区分“键不存在”与其它错误。

---

## 5. 脚本与分布式锁

### 5.1 脚本（script_manager）

支持注册 Lua 脚本并按名称在指定 Redis 实例上执行；执行时优先 EvalSha，失败则回退到完整脚本。组件 Start 时会对所有 Client 预加载已注册脚本。

| 函数 | 说明 |
|------|------|
| `RegisterScript(name string, lua string)` | 注册名为 name 的 Lua 脚本；同名覆盖。在 Add 之前注册，Start 时才会被 loadAllScripts 加载。 |
| `RunScript(rid int, name string, keys []string, args ...interface{}) (interface{}, error)` | 在 id 为 rid 的 Client 上执行已注册脚本 name，传入 keys 与 args；返回脚本执行结果。若 name 未注册或 rid 对应 Client 不存在则返回错误。 |

### 5.2 分布式锁（locker）

基于 Lua 脚本实现 SETNX+EXPIRE 加锁、GET+DEL 校验再解锁；包 init 时已注册 LockScript、UnlockScript。

| 函数 | 说明 |
|------|------|
| `Lock(rid int, key, val string, expire int) error` | 在 rid 对应 Redis 上对 key 加锁，锁值为 val，过期时间 expire 秒；成功返回 nil，已被人持有返回错误。 |
| `LockWithRetry(rid int, key, val string, expire int, option ...retry.Option) error` | 在 Lock 基础上按 lancet/retry 进行重试，option 控制次数与间隔等。 |
| `Unlock(rid int, key, val string) error` | 在 rid 对应 Redis 上解锁 key，仅当当前值等于 val 时删除；校验失败或值不匹配返回错误。 |

使用同一 val 加锁与解锁，避免误删他人锁。

---

## 6. 配置与依赖

### 6.1 配置格式

profile 中键名为组件名（如 "redis"），值为 **数组**，每一项对应一个 Redis 实例，下标即 id（0、1、…）。每项为 Config：`address`（地址列表）、`password`、`pool_size`。

### 6.2 启动与关闭流程

- **Start**：profile.Get → 对每项 Add(i, config) → Range 每 Client loadAllScripts。
- **Stop**：Close()，遍历并关闭所有 Client、清空映射。

### 6.3 依赖

- `github.com/go-redis/redis/v8`
- `github.com/duke-git/lancet/v2/retry`（LockWithRetry）
- `internal/profile`、`internal/iface`、`pkg/lib/component`
