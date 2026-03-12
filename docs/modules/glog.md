# pkg/glog 模块文档

---

## 1. 模块功能概述

`pkg/glog` 基于 zap 提供全局日志：包 init 时自动调用 `Init(DefaultConfig())`，之后可通过 Init 重新初始化、通过 SetLogLevel/GetLevel 调整级别，通过 Debug/Info/…/Fatal 及 Debugf/…/Fatalf 写日志；Stop 用于进程退出前同步缓冲。

---

## 2. 配置（Config）

### 2.1 Config 与 DefaultConfig

| 类型/函数 | 说明 |
|-----------|------|
| `Config` | 配置结构体，含 json/yaml 标签，用于 Init。 |
| `DefaultConfig() *Config` | 返回默认配置：Path "./logs/app.log"、Level "info"、PrintConsole true、MaxSize 500、MaxBackups 100、MaxAge 30、Compress false、LocalTime true。 |

### 2.2 Config 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `Path` | string | 日志文件路径。 |
| `Level` | string | 级别：debug / info / warn / error / dpanic / panic / fatal（不区分大小写）。 |
| `PrintConsole` | bool | 是否同时输出到控制台。 |
| `MaxSize` | int | 单文件最大大小（MB），超过则切割。 |
| `MaxBackups` | int | 最多保留的旧文件数。 |
| `MaxAge` | int | 旧文件保留天数。 |
| `Compress` | bool | 是否压缩旧文件。 |
| `LocalTime` | bool | 是否使用本地时间。 |

---

## 3. 初始化与级别

| 函数 | 说明 |
|------|------|
| `Init(cfg *Config)` | 初始化全局 logger：根据 cfg 设置 AtomicLevel、EncoderConfig、lumberjack 文件 Core，可选控制台 Core；构建 zap.Logger 与 SugaredLogger 并存入包级 atomic.Value。cfg 为 nil 时使用 DefaultConfig()。 |
| `Stop() error` | 对当前 logger 与 sugared 分别 Sync，返回最后一次 Sync 的错误（若有）。进程退出前调用以刷盘。 |
| `SetLogLevel(level zapcore.Level)` | 设置全局日志级别（原子生效）。 |
| `GetLevel() zapcore.Level` | 返回当前全局日志级别。 |
| `WithOptions(opts ...zap.Option)` | 在现有 logger 上应用 zap.Option（如 AddCallerSkip、Hooks），将新 logger 及其 Sugar 存回包级 atomic.Value，后续调用使用新 logger。 |

---

## 4. 结构化输出（msg + fields）

以下函数签名均为 `(msg string, fields ...zap.Field)`，通过包内当前 logger 输出对应级别日志。

| 函数 | 说明 |
|------|------|
| `Debug(msg string, fields ...zap.Field)` | 输出 Debug 级别，不改变程序流程。 |
| `Info(msg string, fields ...zap.Field)` | 输出 Info 级别。 |
| `Warn(msg string, fields ...zap.Field)` | 输出 Warn 级别。 |
| `Error(msg string, fields ...zap.Field)` | 输出 Error 级别。 |
| `Panic(msg string, fields ...zap.Field)` | 输出 Panic 级别并触发 panic；若 logger 未初始化则直接 panic(msg)。 |
| `Fatal(msg string, fields ...zap.Field)` | 输出 Fatal 级别并调用 os.Exit(1)；若 logger 未初始化则直接 os.Exit(1)。 |

---

## 5. 格式化输出（template + args）

以下函数签名均为 `(template string, args ...interface{})`，使用 `fmt.Sprintf` 风格格式化，通过包内当前 SugaredLogger 输出。

| 函数 | 说明 |
|------|------|
| `Debugf(template string, args ...interface{})` | 输出 Debug 级别。 |
| `Infof(template string, args ...interface{})` | 输出 Info 级别。 |
| `Warnf(template string, args ...interface{})` | 输出 Warn 级别。 |
| `Errorf(template string, args ...interface{})` | 输出 Error 级别。 |
| `DPanicf(template string, args ...interface{})` | 输出 DPanic 级别（开发模式下降级为 Error）。 |
| `Panicf(template string, args ...interface{})` | 输出 Panic 级别并触发 panic；logger 未初始化时 panic(Sprintf(template, args...))。 |
| `Fatalf(template string, args ...interface{})` | 输出 Fatal 级别并退出；logger 未初始化时 panic(Sprintf(...))（与 Fatal 的 os.Exit 略有不同，以代码为准）。 |

---

## 6. 依赖

- `go.uber.org/zap`、`go.uber.org/zap/zapcore`
- `gopkg.in/natefinch/lumberjack.v2`（日志文件切割与轮转）
