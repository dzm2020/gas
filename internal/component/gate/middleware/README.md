# Gate 中间件插件

在 `IMiddleware` 基础上提供四种内置插件：限流、压缩、日志、加密。通过 `Agent.AppendMiddleware` 或 `SetMiddleware` 挂载。

## 插件说明

| 插件 | 文件 | 说明 |
|------|------|------|
| **限流** | `ratelimit.go` | 基于 `golang.org/x/time/rate` 令牌桶：**按连接**整连接限流，或**按消息 ID** 仅对指定 Cmd/Act（messageId）限流，其它消息放行；超限返回 `ErrRateLimitExceeded`。 |
| **日志** | `log.go` | 解码后/编码前打 Debug 日志（Cmd/Act/Len/Tag），不修改消息。 |
| **压缩** | `compress.go` | 对 Body 做 gzip 压缩/解压，用 `Head.Tag` 的 `TagCompressed` 位标记。 |
| **加密** | `encrypt.go` | 密钥交换：客户端发 (cmd=0 act=0, clientKey)，服务端回 (0,0, serverKey)；双方用 serverKey+clientKey 派生密钥，Body 与密钥按位异或加解密。 |

## 使用示例

```go
// 按需挂到 Agent 上（如 OnInit 或 Gate 创建 Agent 时）
agent.AppendMiddleware(
    middleware.NewLog(),
    middleware.NewRateLimitForConnection(rate.Limit(1000), 1000), // 按连接限流，每连接 1000/s
    // 或对指定消息 ID 限流（第三个参数为 protocol.CmdAct(cmd, act)）：
    // middleware.NewRateLimitForMessageID(rate.Limit(100), 50, protocol.CmdAct(1, 2)),
    middleware.NewCompress(128),  // 仅当 Body >= 128 字节时压缩
)
// 服务端：NewEncrypt()；收到 (0,0, clientKey) 后回复 (0,0, enc.ServerKey())
enc, _ := middleware.NewEncrypt()
agent.AppendMiddleware(enc)
```

## 顺序建议

- **解码链（AfterDecode）**：先解密 → 再解压 → 再限流/日志（顺序可调）。
- **编码链（BeforeEncode）**：先日志/限流 → 再压缩 → 再加密。

压缩会读写 `protocol.Head.Tag` 的 `TagCompressed` 位；加密使用密钥交换（cmd=0 act=0）与 XOR，无需 Tag 位。
