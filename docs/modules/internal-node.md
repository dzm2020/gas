# internal/node 模块文档

---

## Node 接口说明

Node 为进程内节点抽象：持有 Member 信息、序列化器与组件管理器，对外提供 System、Cluster。通过 `Startup` 完成配置加载、组件注册与启动、集群注册，并阻塞等待退出信号后做优雅关闭。

| 类型/方法 | 说明 |
|-----------|------|
| `New(path string) *Node` | 创建节点实例；path 为配置文件路径。 |
| `Node` | 结构体，持 Member、path、serializer、panicHook 及组件管理器。 |
| `Info() *iface.Member` | 返回节点 Member。 |
| `System() iface.ISystem` | 返回 Actor 系统（从已注册组件中获取）；未注册时返回 nil。 |
| `Cluster() cluster.ICluster` | 返回集群接口（从已注册组件中获取）；未注册时返回 nil。 |
| `Serializer() lib.ISerializer` | 返回当前序列化器。 |
| `SetSerializer(ser lib.ISerializer)` | 设置序列化器。 |
| `SetPanicHook(hook func(zapcore.Entry))` | 设置 panic 时回调。 |
| `Startup(comps ...component.IComponent[iface.INode]) error` | 启动节点：初始化 profile、加载 node 配置、注册并启动内置组件与 comps、在集群中注册本节点、阻塞等待退出信号后停止所有组件并收尾。返回时即已关闭完成。 |
