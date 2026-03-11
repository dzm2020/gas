# internal/profile 模块文档

## 1. 模块功能概述

`internal/profile` 提供基于 Viper 的配置加载：

- **单入口**：`Init(path)` 从指定文件（YAML）加载到包内 viper 实例；后续 `Get(key, cfg)` 按 key 反序列化到 cfg。
- **便捷方法**：`GetCluster()`、`GetLogger()` 读取 cluster/logger 配置并填充默认值；`IsSingleNodeMode()` 读取单节点模式开关。
- 不管理热更新，仅启动时读一次。

## 2. 接口文档

| 函数 | 说明 |
|------|------|
| `Init(path string)` | 设置 ConfigFile、ConfigType("yaml") 并 ReadInConfig；失败时 defer 里 glog.Fatal |
| `Get(key string, cfg interface{}) error` | `vp.UnmarshalKey(key, cfg)` |
| `IsSingleNodeMode() bool` | `viper.GetBool("single-node")` |
| `GetCluster() *cluster.Config` | DefaultConfig() 后 Get("cluster", conf)，失败 Fatal，返回 conf |
| `GetLogger() *logger.Config` | DefaultConfig() 后 Get("logger", conf)，失败 Fatal，返回 conf |

配置文件格式为 YAML，顶层 key 与 `Get(key, cfg)` 对应，例如 `node`、`cluster`、`logger`、`gate`、`redis` 等。

## 3. 设计结构（协程模型与 struct 关系）

### 3.1 协程模型

- 无后台协程；`Init` 与 `Get` 均在调用方 goroutine 中执行。

### 3.2 Struct 关系

- 包内全局 `vp = viper.New()`；依赖 `cluster.Config`、`logger.Config` 的默认值与结构体字段 tag（如 yaml）。
- 其他模块（node、gate、cluster、logger、redis 等）在启动时调用 `profile.Get("key", &config)` 或 `GetCluster()`/`GetLogger()` 获取配置。

### 3.3 依赖

- `github.com/spf13/viper`
- `pkg/cluster`、`pkg/glog`（及 logger 的 Config）
