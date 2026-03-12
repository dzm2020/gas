# 配置与 Profile 组件（原 internal/profile 已移除）

## 1. 概述

项目**不再提供**独立的 `internal/profile` 包。配置加载由节点上的 **Profile 组件**完成，并通过 **`iface.IProfile`** 对外提供接口。节点在启动时把 Profile 作为第一个组件注册，其他组件（Logger、Cluster、System、Gate、Redis 等）在各自的 `Start(ctx, node)` 中通过 **`node.Profile()`** 获取配置。

- **实现位置**：`internal/node/component/profile.go`（Profile 组件实现 `IProfile`）。
- **接口定义**：`internal/iface/node.go` 中的 `IProfile` 与 `INode.Profile()`。
- **单次加载**：Profile 在 `Start` 时从配置文件读入一次，不提供热更新。

## 2. 接口说明

### 2.1 IProfile（internal/iface）

| 方法 | 说明 |
|------|------|
| `Get(key string, cfg interface{}) error` | 将配置中 key 对应内容反序列化到 cfg |
| `GetCluster() *cluster.Config` | 读取 `cluster` 配置并填充默认值；失败时内部 Fatal |
| `GetLogger() *glog.Config` | 读取 `logger` 配置并填充默认值；失败时内部 Fatal |
| `IsSingleNodeMode() bool` | 读取顶层布尔配置 `single-node`，未配置或未加载时为 false |

### 2.2 INode.Profile()

| 方法 | 说明 |
|------|------|
| `Profile() IProfile` | 返回已注册的 Profile 组件（实现 IProfile）；未注册时返回 nil。节点默认组件列表中 Profile 为第一个，故其他组件 Start 时通常非 nil。 |

### 2.3 Profile 组件（internal/node/component）

| 符号 | 说明 |
|------|------|
| `ProfileName` | 常量 `"profile"`，组件名与 `node.GetComponent(ProfileName)` 一致 |
| `NewProfile(path string) *Profile` | 创建 Profile 组件，path 为配置文件路径（如节点 config 路径） |
| `(*Profile) Name() string` | 返回 `ProfileName` |
| `(*Profile) Start(ctx, node) error` | 用 viper 从 `path` 读配置，并将 `"node"` 反序列化到 `node.Info()`；读文件失败时 Fatal |

配置文件格式为 YAML（或 viper 支持的其他格式），顶层 key 与 `Get(key, cfg)` 对应，例如 `node`、`cluster`、`logger`、`gate`、`redis` 等。

## 3. 设计结构

### 3.1 组件与 viper

- **Profile** 结构体持有 `path string` 与 `vp *viper.Viper`；`vp` 在 `Start` 中通过 `viper.New()` 创建并 `SetConfigFile(path)`、`ReadInConfig()`，此后 `Get`/`GetCluster`/`GetLogger`/`IsSingleNodeMode` 均基于该实例，无全局 viper。
- Profile 嵌入 `component.BaseComponent[iface.INode]`，实现 `IComponent[iface.INode]` 与 `IProfile`。

### 3.2 使用方式

- **节点**：在 `Startup` 中把 `com.NewProfile(n.path)` 作为默认组件列表的第一项注册，再注册 Logger、Cluster、System 等。
- **其他组件**：在 `Start(ctx context.Context, node iface.INode) error` 中通过 `node.Profile()` 取配置，例如：
  - Logger：`node.Profile().GetLogger()`
  - Cluster：`node.Profile().IsSingleNodeMode()`、`node.Profile().GetCluster()`
  - System：`node.Profile().IsSingleNodeMode()`
  - Gate：`node.Profile().Get(r.Name(), conf)`
  - Redis：`node.Profile().Get(c.Name(), &configs)`

### 3.3 依赖

- Profile 组件依赖：`github.com/spf13/viper`、`pkg/cluster`、`pkg/glog`、`internal/iface`、`pkg/lib/component`。
- IProfile 定义在 iface 中，依赖 `pkg/cluster`、`pkg/glog`。
