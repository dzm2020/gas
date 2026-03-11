// Package protocol 定义网关二进制协议：固定 13 字节头（Len/Cmd/Act/Error/Index/Tag）+ 变长 Body。
package protocol

const HeadLen = 13

// New
//
//	@Description: 构造协议消息，Len 由 codec 按 Data 长度写入。
//	@param cmd
//	@param act
//	@param data
//	@return *Message
func New(cmd, act uint8, data []byte) *Message {
	h := &Head{}
	h.SetCmd(cmd)
	h.SetAct(act)
	return &Message{Head: h, Data: data}
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
	m.SetIndex(old.GetIndex())
	m.SetCmd(old.GetCmd())
	m.SetAct(old.GetAct())
	m.SetTag(old.GetTag())
}

// ID
//
//	@Description: 返回 Cmd<<8+Act 的组合 ID。
//	@receiver m
//	@return uint16
func (m *Message) ID() uint16 {
	return CmdAct(m.GetCmd(), m.GetAct())
}

// Head 协议头，成员小写仅包内访问，对外通过 Get/Set。
type Head struct {
	len     uint32 // 包体长度 4
	cmd     uint8  // 命令 1
	act     uint8  // 动作 1
	errCode uint16 // 错误码 2
	index   uint32 // 序号 4
	tag     uint8  // 标签 1，由业务或中间件使用
}

func (h *Head) GetLen() uint32    { return h.len }
func (h *Head) SetLen(v uint32)   { h.len = v }
func (h *Head) GetCmd() uint8     { return h.cmd }
func (h *Head) SetCmd(v uint8)    { h.cmd = v }
func (h *Head) GetAct() uint8     { return h.act }
func (h *Head) SetAct(v uint8)    { h.act = v }
func (h *Head) GetError() uint16  { return h.errCode }
func (h *Head) SetError(v uint16) { h.errCode = v }
func (h *Head) GetIndex() uint32  { return h.index }
func (h *Head) SetIndex(v uint32) { h.index = v }
func (h *Head) GetTag() uint8     { return h.tag }
func (h *Head) SetTag(v uint8)    { h.tag = v }

// Clone 返回 Head 的副本，供中间件修改 Tag 等字段后生成新 Message。
func (h *Head) Clone() *Head {
	if h == nil {
		return nil
	}
	return &Head{len: h.len, cmd: h.cmd, act: h.act, errCode: h.errCode, index: h.index, tag: h.tag}
}

// NewDecoded 由 codec 解码时构造消息，包外仅通过此函数或 New 创建带 Head 的 Message。
func NewDecoded(bodyLen uint32, cmd, act uint8, errCode uint16, index uint32, tag uint8, data []byte) *Message {
	h := &Head{}
	h.SetLen(bodyLen)
	h.SetCmd(cmd)
	h.SetAct(act)
	h.SetError(errCode)
	h.SetIndex(index)
	h.SetTag(tag)
	return &Message{Head: h, Data: data}
}

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
