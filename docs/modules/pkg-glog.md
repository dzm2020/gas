# pkg/glog 模块文档

## 1. 模块功能概述

`pkg/glog` 提供基于 zap 的全局日志：

- **初始化**：Init(cfg) 设置 AtomicLevel、EncoderConfig、lumberjack 与多 Core（文件 + 可选控制台），init() 时默认调用 Init(DefaultConfig())。
- **级别**：SetLogLevel、GetLevel；Debug/Info/Warn/Error/Panic/Fatal 及 Debugf/Infof/.../Fatalf。
- **行为**：Panic/Fatal 会 panic 或 os.Exit；Stop() 同步缓冲。
- **配置**：Config 含 Path、MaxSize、MaxBackups、MaxAge、LocalTime、Compress、PrintConsole、Level 等，见 config.go。

## 2. 接口文档

### 2.1 初始化与配置

| 函数 | 说明 |
|------|------|
| `Init(cfg *Config)` | 构建 zap Logger 与 SugaredLogger，存于 atomic.Value；cfg 为 nil 时用 DefaultConfig() |
| `DefaultConfig() *Config` | 默认文件与控制台、级别等 |
| `Stop() error` | Sync 当前 logger 与 sugared |

### 2.2 级别

| 函数 | 说明 |
|------|------|
| `SetLogLevel(level zapcore.Level)` | 设置级别 |
| `GetLevel() zapcore.Level` | 当前级别 |

### 2.3 输出

| 函数 | 说明 |
|------|------|
| `Debug/Info/Warn/Error(msg string, fields ...zap.Field)` | 结构化 |
| `Panic/Fatal(...)` | Panic 触发 panic，Fatal 调用 os.Exit(1) |
| `Debugf/Infof/Warnf/Errorf/DPanicf/Panicf/Fatalf(template string, args ...interface{})` | 格式化 |

### 2.4 扩展

| 函数 | 说明 |
|------|------|
| `WithOptions(opts ...zap.Option)` | 替换当前 logger |

## 3. 设计结构（协程模型与 struct 关系）

### 3.1 协程模型

- 无包内常驻 goroutine；写日志由调用方 goroutine 执行，zap 内部有缓冲与异步写（视配置而定）。
- atomic.Value 存储 logger 指针，Init/WithOptions 时替换，并发读安全。

### 3.2 Struct 关系

```
包级变量:
  loggerValue, sugaredValue atomic.Value  (*zap.Logger, *zap.SugaredLogger)
  atomicLevel zap.AtomicLevel

Init(cfg) -> 构建 cores (文件 + 可选控制台) -> zap.New -> Store
```

### 3.3 依赖

- go.uber.org/zap、zapcore
- gopkg.in/natefinch/lumberjack.v2
