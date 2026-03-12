# Agent 关闭流程说明（主动关闭 vs 被动关闭）

## 1. 两条关闭路径

### 1.1 主动关闭（业务发起）

触发：业务调用 `session.Close()`，经 Transport 发往 Agent 的 `Shutdown` 方法。

```
Session.Close()
  → transport.Close()
  → send(agent, "Shutdown", nil)
  → Agent 收到 Shutdown 消息，执行 Agent.Shutdown(ctx, nil)
      ① agent.GetEntity().Close(nil)     // 关闭网络连接
      ② ctx.Shutdown()                   // 关闭当前 Actor 进程
```

**实际调用链细节：**

1. ① `entity.Close(nil)` 进入网络层 `baseConn.Close(connection, err)`：
   - `OnClose(connection, err)` 被**同步**调用 → **Gate.OnClose(entity, nil)**
   - Gate：`count.Add(-1)`，`ShutdownProcess(pid)`
   - `ShutdownProcess(pid)` → `process.Shutdown()` → `Stop()` + `mailbox.PostMessage(exitTask)`（直接投递到 mailbox，不经过 Process.PostMessage，故不受 IsStop 限制，退出任务能入队）
2. 然后 `RemoveConnection(connection)`、`cancel()`，`entity.Close` 返回。
3. ② `ctx.Shutdown()` → 再次 `process.Shutdown()`，此时 `Stop()` 已为 true，直接 return，不再投递。
4. 当前 Shutdown 处理函数返回后，mailbox 继续执行已入队的 **exitTask**：`Unregister(ctx)`、`ctx.Actor().OnStop(ctx)`。

**结论：** 主动关闭路径下，OnStop 会被调用，顺序正确；count 只减一次；无重复投递退出任务的问题。

---

### 1.2 被动关闭（网络断开）

触发：客户端断线、超时、网络异常等，由网络层检测到连接关闭后调用 `handler.OnClose(conn, err)`。

```
网络层检测到连接关闭
  → baseConn.Close(connection, err) 或等价逻辑
  → OnClose(connection, err)
  → Gate.OnClose(entity, err)
      ① count.Add(-1)
      ② ShutdownProcess(pid)
  → process.Shutdown() → Stop() + mailbox.PostMessage(exitTask)
  → 后续 mailbox 执行 exitTask → Unregister + Agent.OnStop(ctx)
```

此时 **entity 已由网络层关闭**，Agent 内不需要、也不应再对 entity 做 Close。OnStop 中若访问 `GetEntity()`，应能容忍连接已关闭或即将被回收的状态。

**结论：** 被动关闭路径下，仅由 Gate 侧 ShutdownProcess 驱动进程退出，OnStop 会被调用，行为正确。

---

## 2. 设计要点

| 项目 | 说明 |
|------|------|
| **Process.Shutdown 的退出任务** | `process.Shutdown()` 中调用的是 `p.mailbox.PostMessage(exitTask)`，不经过 `Process.PostMessage`，因此不会因 `IsStop()==true` 而拒绝投递，退出任务一定能入队。 |
| **主动关闭时的“双重”Shutdown** | 先由 `entity.Close` 同步触发 Gate.OnClose → ShutdownProcess（第一次，成功投递 exitTask）；再在 Agent 内执行 `ctx.Shutdown()`（第二次，Stop 已 true 直接返回）。第二次为幂等，无副作用。 |
| **count 准确性** | 两种路径下 count 都只在 Gate.OnClose 中减 1，不会重复减。 |

---

## 3. 建议与注意事项

1. **Gate.OnClose 中 ShutdownProcess 前做 nil 检查**  
   `GetProcess(pid)` 在极端情况下可能返回 nil（例如进程已 Unregister 或 pid 陈旧）。建议先判断 `process != nil` 再调用 `ShutdownProcess`，避免 panic。
2. **OnStop 中对 entity 的使用**  
   被动关闭时，OnStop 可能在连接已关闭或已从连接表移除之后执行，业务在 OnStop 里使用 `GetEntity()`（如打日志）应能容忍“已关闭”状态，避免假设连接仍可读写。
3. **主动关闭时保留 ctx.Shutdown()**  
   Agent.Shutdown 中先 `entity.Close()` 再 `ctx.Shutdown()` 是合理的；保留 `ctx.Shutdown()` 可使“仅调 ctx.Shutdown 而不经 Session.Close”的用法也能正确退出，与当前设计一致。
