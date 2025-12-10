// Package iface 定义 Actor 模型的核心接口
// 包括 Actor 上下文、Actor 实例、进程、系统和路由器等接口定义
package iface

import (
	"gas/pkg/lib"
	"time"
)

// Task Actor 任务函数类型，用于在 Actor 上下文中执行异步任务
// ctx: Actor 上下文，提供消息发送、定时器等能力
// 返回: 执行错误，nil 表示成功
type Task func(ctx IContext) error

// IContext Actor 上下文接口，提供 Actor 运行时的核心能力
// 包括消息发送、接收、定时器管理、路由等功能
type IContext interface {
	// ID 获取当前 Actor 的进程 ID
	ID() *Pid

	InvokerMessage(msg interface{}) error

	// Actor 获取当前 Actor 实例
	Actor() IActor

	// Send 异步发送消息到指定进程
	// to: 目标进程 ID
	// msgId: 消息 ID
	// request: 请求数据，可以是任意类型，会自动序列化
	// 返回: 发送错误，nil 表示成功
	Send(to *Pid, msgId uint16, request interface{}) error

	// Call 同步发送请求消息并等待响应
	// to: 目标进程 ID
	// msgId: 消息 ID
	// request: 请求数据
	// reply: 响应数据指针，用于接收反序列化后的响应
	// 返回: 请求错误，nil 表示成功
	Call(to *Pid, msgId uint16, request interface{}, reply interface{}) error

	// GetSerializer 获取序列化器，用于消息的序列化和反序列化
	GetSerializer() lib.ISerializer

	// AfterFunc 注册一次性定时器
	// duration: 延迟时间
	// callback: 定时器到期后的回调函数
	// 返回: 定时器
	AfterFunc(duration time.Duration, callback Task) *lib.Timer

	// TickFunc 注册周期性定时器，每隔指定时间间隔执行一次回调
	// interval: 执行间隔时间
	// callback: 每次执行的回调函数
	// 返回: 定时器
	TickFunc(interval time.Duration, callback Task) *lib.Timer

	// RegisterName 注册进程名称
	// name: 进程名称
	// isGlobal: 是否全局注册（跨节点可见）
	// 返回: 注册错误
	RegisterName(name string, isGlobal bool) error

	// SetRouter 设置消息路由器
	// router: 路由器实例
	SetRouter(router IRouter)

	// GetRouter 获取消息路由器
	GetRouter() IRouter

	// Message 获取当前正在处理的消息
	// 返回: 当前消息，如果没有则返回 nil
	Message() *Message

	// System 获取 Actor 系统实例
	System() ISystem

	// Shutdown 退出当前 Actor，清理资源并触发 OnStop 回调
	Shutdown() error

	Process() IProcess
}

// IActor Actor 接口，定义 Actor 的生命周期和消息处理
type IActor interface {
	// OnInit Actor 初始化回调，在 Actor 创建后立即调用
	// ctx: Actor 上下文
	// params: 初始化参数，由 Spawn 方法传入
	// 返回: 初始化错误，nil 表示成功
	OnInit(ctx IContext, params []interface{}) error

	// OnMessage 处理接收到的消息
	// ctx: Actor 上下文
	// msg: 接收到的消息
	// 返回: 处理错误，nil 表示成功
	OnMessage(ctx IContext, msg *Message) error

	// OnStop Actor 停止回调，在 Actor 退出前调用
	// ctx: Actor 上下文
	// 返回: 停止错误，nil 表示成功
	OnStop(ctx IContext) error
}

// IProcess Actor 进程接口，管理 Actor 的消息队列和执行
type IProcess interface {
	// Context 获取进程的上下文
	Context() IContext

	// PushTask 推送任务到进程的消息队列（异步执行）
	// f: 要执行的任务函数
	// 返回: 推送错误，nil 表示成功
	PushTask(f Task) error

	// PushTaskAndWait 推送任务并等待执行完成（同步执行）
	// timeout: 等待超时时间
	// task: 要执行的任务函数
	// 返回: 执行错误，nil 表示成功
	PushTaskAndWait(timeout time.Duration, task Task) error

	// Send
	//  @Description:异步调用
	//  @param message
	//  @return error
	Send(message *Message) error

	// Call
	//  @Description:
	//  @param message
	//  @param timeout
	//  @return *RespondMessage
	Call(message *Message, timeout time.Duration) *Response

	// Shutdown 退出进程，清理资源并等待所有任务完成
	// 返回: 退出错误，nil 表示成功
	Shutdown() error
}

// ISystem Actor 系统接口，管理所有 Actor 进程和消息传递
type ISystem interface {
	// SetNode 设置节点实例
	// n: 节点实例
	SetNode(n INode)

	// GetNode 获取节点实例
	GetNode() INode

	// GetSerializer 获取序列化器
	GetSerializer() lib.ISerializer

	// SetSerializer 设置序列化器
	// ser: 序列化器实例
	SetSerializer(ser lib.ISerializer)

	// Spawn 创建新的 Actor 进程
	// actor: Actor 实例
	// args: 初始化参数，会传递给 Actor.OnInit
	// 返回: 新创建的进程 ID
	Spawn(actor IActor, args ...interface{}) *Pid

	// Send 发送消息到指定进程（异步）
	// message: 消息对象
	// 返回: 发送错误，nil 表示成功
	Send(message *Message) error

	// Call 发送请求消息并等待响应（同步）
	// message: 请求消息
	// timeout: 超时时间
	// 返回: 响应消息
	Call(message *Message, timeout time.Duration) *Response

	// PushTask 推送任务到指定进程（异步）
	// pid: 目标进程 ID
	// f: 任务函数
	// 返回: 推送错误
	PushTask(pid *Pid, f Task) error

	// PushTaskAndWait 推送任务到指定进程并等待完成（同步）
	// pid: 目标进程 ID
	// timeout: 等待超时时间
	// task: 任务函数
	// 返回: 执行错误
	PushTaskAndWait(pid *Pid, timeout time.Duration, task Task) error

	// Select 根据名称和路由策略选择进程
	// name: 进程名称
	// strategy: 路由策略
	// 返回: 进程 ID 和错误
	Select(name string, strategy RouteStrategy) (*Pid, error)

	// RegisterName 注册进程名称
	// pid: 进程 ID
	// process: 进程实例
	// name: 进程名称
	// isGlobal: 是否全局注册
	// 返回: 注册错误
	RegisterName(pid *Pid, process IProcess, name string, isGlobal bool) error
}

// IRouter 消息路由器接口，用于注册和处理消息处理器
type IRouter interface {
	// Register 注册消息处理器
	// msgId: 消息 ID
	// handler: 处理器函数，支持以下三种签名：
	//   - 客户端消息: func(ctx IContext, session ISession, request *RequestType) error
	//   - 同步消息: func(ctx IContext, request *RequestType, response *ResponseType) error
	//   - 异步消息: func(ctx IContext, request *RequestType) error
	// 注意: 如果注册失败，错误会通过日志输出
	Register(msgId int64, handler interface{})

	// Handle 处理消息
	// ctx: Actor 上下文
	// msgId: 消息 ID
	// session: 客户端会话（客户端消息时不为 nil）
	// data: 消息数据（已序列化的字节数组）
	// 返回: 响应数据（同步消息）和错误，异步调用时 response 为 nil
	Handle(ctx IContext, msgId int64, session ISession, data []byte) ([]byte, error)

	// HasRoute 判断指定消息ID的路由是否存在
	// msgId: 消息 ID
	// 返回: 是否存在路由
	HasRoute(msgId int64) bool
}

// Actor 基础 Actor 实现，提供默认的空实现
// 可以作为其他 Actor 的基类，通过嵌入来继承默认行为
var _ IActor = (*Actor)(nil)

type Actor struct {
}

// OnInit 初始化回调（空实现）
func (a *Actor) OnInit(ctx IContext, params []interface{}) error {
	return nil
}

// OnStop 停止回调（空实现）
func (a *Actor) OnStop(ctx IContext) error {
	return nil
}

// OnMessage 消息处理回调（空实现）
func (a *Actor) OnMessage(ctx IContext, msg *Message) error {
	return nil
}
