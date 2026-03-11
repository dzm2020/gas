# pkg/discovery 模块文档

## 1. 模块功能概述

`pkg/discovery` 提供服务发现抽象与实现：

- **IDiscovery 接口**：Run、Register、Update、Deregister、GetById/GetByKind/GetAll/GetByTag、Watch/Unwatch、Shutdown。
- **配置驱动**：Config 含 Type（如 "consul"）与 Config map；NewFromConfig 通过 registry 按 Type 创建实现。
- **成员与拓扑**：Member 含 Id、Kind、Address、Port、Tags、Meta、Status；MemberList、Topology 用于对比 Joined/Left/Update。
- **provider/consul**：通过 init 注册到 registry，实现基于 Consul 的发现与注册。

## 2. 接口文档

### 2.1 iface

| 接口/类型 | 说明 |
|-----------|------|
| `IDiscovery` | Run、Register、Update、Deregister、GetById、GetByKind、GetAll、GetByTag、Watch、Unwatch、Shutdown |
| `ServiceChangeHandler` | `func(*Topology)` |
| `MemberList` | Dict map[uint64]*Member；UpdateTopology(old) 得到 Topology |
| `Topology` | All、Update、Joined、Left；IsChange() bool |

### 2.2 Member（iface.Member 或 discovery/iface）

| 字段/方法 | 说明 |
|-----------|------|
| Id、Kind、Address、Port、Tags、Meta、Status | 成员信息 |
| GetKind/GetID/GetAddress/GetPort/GetTags/GetMeta/GetStatus | 访问器 |
| Equal(other *Member) bool | 相等比较 |

### 2.3 discovery 包

| 函数/类型 | 说明 |
|-----------|------|
| `Config` | Type、Config map[string]interface{} |
| `NewFromConfig(config Config) (IDiscovery, error)` | 从 registry 按 Type 取工厂并创建 |
| `init.go` | import _ provider/consul 以注册 |

### 2.4 provider/consul

由 init 注册类型 "consul"；实现 IDiscovery（Consul API 注册、健康检查、Watch 等）。具体 API 见 consul 包内。

## 3. 设计结构（协程模型与 struct 关系）

### 3.1 协程模型

- **Run**：实现内可能起 goroutine 做健康检查、拉取或监听 Consul。
- **Watch**：回调通常在发现实现的 goroutine 中调用，调用方需注意并发安全。

### 3.2 Struct 关系

```
Config (Type + Config map)
  -> NewFromConfig -> registry.Get(type) -> creator(Config) -> IDiscovery

Member: 平面结构，Id/Kind/Address/Port/Tags/Meta/Status

MemberList.Dict: map[uint64]*Member
Topology: All, Update, Joined, Left
```

### 3.3 依赖

- `pkg/discovery/iface`、`pkg/discovery/registry`、`pkg/discovery/provider/consul`
- 第三方：hashicorp/consul/api 等
