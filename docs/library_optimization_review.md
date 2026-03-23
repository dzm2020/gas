# GAS 库级审查与优化建议

本文档基于对仓库 `github.com/dzm2020/gas` 的静态浏览与 `go test ./...` 结果整理，按优先级列出可改进项，便于排期与跟踪。**未改业务代码**，仅作建议清单。

---

## 1. 摘要

| 类别 | 结论 |
|------|------|
| **测试健康** | 主模块默认 `go test ./...` 已对齐（集群相关集成用例见 `-tags=integration`）；`tools/discard` 已拆为独立子模块，不再参与根目录 `./...`。 |
| **模块边界** | 示例与业务大量引用 `internal/*`，作为可 `go get` 的库时，外部项目无法复用相同导入路径；若定位为「应用框架」需在文档中明确。 |
| **依赖** | 主要依赖较新；`go-redis/v8` 已进入维护态，长期可考虑迁移 v9。 |
| **工程卫生** | `.gitignore` 规则较粗；测试或运行产生的日志目录可能误入版本库。 |

---

## 2. 高优先级：修复构建与测试一致性

### 2.1 `pkg/cluster` 单测与 `ICluster.Subscribe` 签名不一致（已修复）

`ICluster.Subscribe` 现为 **`(subject string, subscriber ...)`**（见 `pkg/cluster/cluster.go`），用于节点收件箱 subject（如 `"1"`）及事件 subject（如 `gas.event.>`）。`pkg/cluster/cluster_test.go` 中仍传入**整型字面量** `1`、`2`，导致编译失败。

**建议**：将 `c.Subscribe(1, sub)` 改为 `c.Subscribe("1", sub)` 等形式，与 `Send`/`Call` 使用的节点 ID 字符串约定一致。

### 2.2 `examples/actor` 与 `ISystem.Call` 签名不一致（已修复）

`System.Call` 签名为：

`Call(from, to *Pid, methodName string, request, reply interface{}, timeout time.Duration) error`

`examples/actor/actor_test.go` 中仍为旧式少参调用（缺少 `from`、`timeout`）。

**建议**：传入合法 `from`（例如再 Spawn 一个 Actor 取其 `Pid`，或使用测试用占位 Pid），并传入明确超时（如 `2*time.Second`）。

### 2.3 `examples/cluster/client` 集成测试（已用 `integration` 标签隔离）

`TestClient` 固定连接 `127.0.0.1:9002`，无 Gate 进程时必然失败。

**建议（任选）**：

- 使用 **`testing.Short()`** 或 **build tag**（如 `//go:build integration`）在未启动依赖时跳过；或  
- 在测试中拉起子进程/用 `t.Skip` 说明前置条件；或  
- 从默认 `go test ./...` 路径中排除该包（文档说明单独跑集成测试的命令）。

`examples/cluster/gate-node` 中阻塞在 `Node.Startup` 的用例同样使用 **`//go:build integration`**；包内保留无标签的 `doc.go`，避免默认构建下出现「无 Go 文件」包错误。

### 2.4 `tools/discard` 归档代码（已拆子模块）

已在 `tools/discard/go.mod` 将本目录设为**独立子模块**（`replace` 指向仓库根），根目录 `go test ./...` 不再编译该树。若需维护其中代码，在 `tools/discard` 下执行 `go mod tidy` / `go build ./...`。

---

## 3. 模块边界与对外 API

### 3.1 `internal` 与示例代码

`examples/*` 普遍依赖 `internal/actor`、`internal/node`、`internal/component/*` 等。Go 规则下，**其他模块无法导入本仓库的 `internal`**，因此公开 README 中的「安装 `go get`」路径，与「示例即最佳实践」之间存在张力。

**建议**：

- 在 README / `docs/lib.md` 中明确：本仓库当前以**单模块应用 + 拷贝式复用**为主，或  
- 将希望第三方直接使用的类型与构造函数逐步迁到 **`pkg/`**（或顶层稳定包），`internal` 仅保留实现细节。

### 3.2 两套「事件」命名

- **`internal/actor` + `IContext.Subscribe`**：Actor 邮箱线程模型、topic + `[]byte`。  
- **`pkg/lib/event`**：`Listener` 泛型、进程内同步回调列表。

名称相近但语义不同，新同学易混淆。

**建议**：在 `docs/event.md` 中加一节「与 `pkg/lib/event` 的区别」；或考虑将其中一方改名（如 `pkg/lib/event` → `signal` / `hook`），属破坏性变更需版本说明。

---

## 4. 依赖与运行时

- **`github.com/go-redis/redis/v8`**：官方主推 v9；v8 仍可维护期使用，中长期计划迁移可减少安全与兼容风险。  
- **`go 1.25.3`**（`go.mod`）：若团队环境尚未统一，建议在 README 注明「最低工具链」与 CI 镜像版本，避免本地与 CI 行为不一致。  
- **Consul / NATS**：集群与事件相关测试依赖本机服务；已在部分测试中使用 `Skip`，建议统一策略并在 `docs` 中写清「集成测试前置条件」。

---

## 5. 代码与性能（非必须，按需采纳）

### 5.1 `pkg/network/manager.go` — `ConnManager.GetAll`

当前实现先 `RLock` 收集 keys、释放后再 `RLock` 组装切片，意在缩短持锁时间，但两次读锁之间连接可能被删增，语义为「尽力一致快照」。若连接规模不大，**单次 `RLock` 内复制**可能更简单且仍足够快；若规模很大，可再评估无锁快照或版本号方案。

### 5.2 事件总线 `PublishLocal`

对每个订阅者复制 `payload` 并在各自 Actor 邮箱执行回调，有利于隔离与线程安全；高 QPS、大 payload 场景可考虑文档说明「宜小消息」或引入共享缓冲/零拷贝策略（需严格约定只读与生命周期）。

### 5.3 `pkg/lib/event` 中 `reflect` 比较 handler

`handlerComparable` 依赖函数指针比较，对闭包重复注册等行为需文档说明；若热路径敏感可评估是否改为显式 `id` 注销模型。

---

## 6. 仓库与工程化

- **`.gitignore`**：当前包含 `*examples`、`*tools` 等模式，可能误伤路径名含这些子串的文件；建议改为显式目录（如 `/examples/` 若确需忽略）或注释说明意图，避免与「示例应入仓」冲突。  
- **测试产物**：例如 `pkg/network/logs/` 下若有运行产生的 `app.log`，应加入 `.gitignore` 并勿提交。  
- **CI**：建议至少执行 `go vet ./...`、`go test` 对**核心包**（如 `./internal/...`、`./pkg/...`、选定 `examples/...`）固定子集，避免因 `tools` 或集成测试导致整条流水线红。  
- **Protobuf**：`internal/pb` 与 `actor.proto` 变更时，在贡献说明中固定 `protoc`/插件版本与生成命令，减少生成文件漂移。

---

## 7. 文档与可观测性

- 已有 `docs/message_flow.md`、`docs/event.md`、`docs/config.md` 等，可在 README 的「文档索引」中增加一页总表链接。  
- Gate 中间件（加密、限流、压缩）涉及密钥与配额，建议在 `docs/config.md` 或安全小节中说明**勿将密钥写入仓库**、生产配置注入方式。  
- 关键错误路径已使用 `glog`/zap；可对跨节点失败、事件投递失败汇总指标（若后续引入 metrics），便于运维。

---

## 8. 建议执行顺序（参考）

1. 修复 `pkg/cluster/cluster_test.go`、`examples/actor/actor_test.go` 的编译错误，使 `go test ./pkg/... ./internal/...` 稳定绿。  
2. 为 `examples/cluster/client` 与 `tools/discard` 划定测试范围（skip / tag / 移出默认测试集）。  
3. 清理误提交的日志、收紧 `.gitignore`。  
4. 文档澄清 `internal` 使用范围与集成测试依赖。  
5. 中长期：公开 API 下沉、`redis` 大版本、命名区分两类事件。

---

*文档生成自仓库审查；具体行号与接口以当前主分支为准。*
