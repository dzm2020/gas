// Package protocol 定义网关二进制协议：固定 12 字节头（Len/Cmd/Act/Error/Index）+ 变长 Body。
package protocol

const HeadLen = 12 // 协议头长度（字节）：Len(4)+Cmd(1)+Act(1)+Error(2)+Index(4)

// New 构造一条协议消息，Head 中 Len 初始为 0（由 codec 编码时按 Data 长度写入）。
func New(cmd, act uint8, data []byte) *Message {
	return &Message{
		Head: &Head{
			Len:   0,
			Cmd:   cmd,
			Act:   act,
			Error: 0,
			Index: 0,
		},
		Data: data,
	}
}

// NewData 构造 Cmd=0、Act=0 的纯数据消息，常用于业务透传或 Response。
func NewData(data []byte) *Message {
	return New(0, 0, data)
}

// NewErr 构造仅带错误码的响应消息，Body 为空。
func NewErr(err uint16) *Message {
	msg := New(0, 0, nil)
	msg.SetError(err)
	return msg
}

// Message 协议消息：内嵌 Head，Data 为包体。
type Message struct {
	*Head
	Data []byte
}

// Copy 从 old 复制 Cmd、Act、Index 到当前消息（用于回包时保持序号与路由信息）。
func (m *Message) Copy(old *Message) {
	if old == nil {
		return
	}
	m.Index = old.Index
	m.Cmd = old.Cmd
	m.Act = old.Act
}

// ID 返回 Cmd<<8+Act 的组合 ID，用于路由或映射。
func (m *Message) ID() uint16 {
	return CmdAct(m.Cmd, m.Act)
}

type Head struct {
	Len   uint32 // 包体长度 4
	Cmd   uint8  // 命令 1
	Act   uint8  // 动作 1
	Error uint16 // 错误码 2
	Index uint32 // 序号 4
}

func (h *Head) GetLen() uint32    { return h.Len }
func (h *Head) SetLen(v uint32)   { h.Len = v }
func (h *Head) GetCmd() uint8     { return h.Cmd }
func (h *Head) SetCmd(v uint8)    { h.Cmd = v }
func (h *Head) GetAct() uint8     { return h.Act }
func (h *Head) SetAct(v uint8)    { h.Act = v }
func (h *Head) GetError() uint16  { return h.Error }
func (h *Head) SetError(v uint16) { h.Error = v }
func (h *Head) GetIndex() uint32  { return h.Index }
func (h *Head) SetIndex(v uint32) { h.Index = v }

// CmdAct 将 cmd、act 合并为 16 位 ID（高 8 位 cmd，低 8 位 act）。
func CmdAct(cmd, act uint8) uint16 {
	return uint16(cmd)<<8 + uint16(act)
}

// ParseId 将 16 位 msgId 拆成 cmd（高 8 位）与 act（低 8 位）。
func ParseId(msgId uint16) (uint8, uint8) {
	cmd := uint8(msgId >> 8)
	act := uint8(msgId & 0xFF)
	return cmd, act
}
