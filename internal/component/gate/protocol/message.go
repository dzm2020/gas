// Package protocol 定义网关二进制协议：固定 13 字节头（Len/Cmd/Act/Error/Index/Tag）+ 变长 Body。
package protocol

const HeadLen = 13 // 协议头长度（字节）：Len(4)+Cmd(1)+Act(1)+Error(2)+Index(4)+Tag(1)

// New
//
//	@Description: 构造协议消息，Len 由 codec 按 Data 长度写入。
//	@param cmd
//	@param act
//	@param data
//	@return *Message
func New(cmd, act uint8, data []byte) *Message {
	return &Message{
		Head: &Head{
			Len:   0,
			Cmd:   cmd,
			Act:   act,
			Error: 0,
			Index: 0,
			Tag:   0,
		},
		Data: data,
	}
}

// NewData
//
//	@Description: 构造 (0,0) 纯数据消息。
//	@param data
//	@return *Message
func NewData(data []byte) *Message {
	return New(0, 0, data)
}

// NewErr
//
//	@Description: 构造仅带错误码的响应，Body 为空。
//	@param err
//	@return *Message
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

// Copy
//
//	@Description: 从 old 复制 Cmd、Act、Index、Tag，用于回包。
//	@receiver m
//	@param old
func (m *Message) Copy(old *Message) {
	if old == nil {
		return
	}
	m.Index = old.Index
	m.Cmd = old.Cmd
	m.Act = old.Act
	m.Tag = old.Tag
}

// ID
//
//	@Description: 返回 Cmd<<8+Act 的组合 ID。
//	@receiver m
//	@return uint16
func (m *Message) ID() uint16 {
	return CmdAct(m.Cmd, m.Act)
}

type Head struct {
	Len   uint32 // 包体长度 4
	Cmd   uint8  // 命令 1
	Act   uint8  // 动作 1
	Error uint16 // 错误码 2
	Index uint32 // 序号 4
	Tag   uint8  // 标签 1，由业务或中间件使用
}

func (h *Head) GetLen() uint32    { return h.Len }
func (h *Head) SetLen(v uint32)   { h.Len = v }
func (h *Head) GetCmd() uint8     { return h.Cmd }
func (h *Head) SetCmd(v uint8)    { h.Cmd = v }
func (h *Head) GetAct() uint8     { return h.Act }
func (h *Head) SetAct(v uint8)    { h.Act = v }
func (h *Head) GetError() uint16  { return h.Error }
func (h *Head) SetError(v uint16) { h.Error = v }
func (h *Head) GetIndex() uint32 { return h.Index }
func (h *Head) SetIndex(v uint32) { h.Index = v }
func (h *Head) GetTag() uint8     { return h.Tag }
func (h *Head) SetTag(v uint8)    { h.Tag = v }

// CmdAct
//
//	@Description: 将 cmd、act 合并为 16 位 ID。
//	@param cmd
//	@param act
//	@return uint16
func CmdAct(cmd, act uint8) uint16 {
	return uint16(cmd)<<8 + uint16(act)
}

// ParseId
//
//	@Description: 将 msgId 拆成 cmd 与 act。
//	@param msgId
//	@return uint8
//	@return uint8
func ParseId(msgId uint16) (uint8, uint8) {
	cmd := uint8(msgId >> 8)
	act := uint8(msgId & 0xFF)
	return cmd, act
}
