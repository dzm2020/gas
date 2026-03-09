package protocol

const HeadLen = 13

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

type Message struct {
	*Head
	Data []byte
}

func (m *Message) Copy(old *Message) {
	if old == nil {
		return
	}
	m.Index = old.Index
	m.Cmd = old.Cmd
	m.Act = old.Act
}
func (m *Message) ID() uint16 {
	return CmdAct(m.Cmd, m.Act)
}

type Head struct {
	Len   uint32 // 包体长度
	Cmd   uint8  // 命令
	Act   uint8  // 动作
	Error uint16 // 错误码
	Index uint32 // 序号
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

func CmdAct(cmd, act uint8) uint16 {
	return uint16(cmd)<<8 + uint16(act)
}

func ParseId(msgId uint16) (uint8, uint8) {
	cmd := uint8(msgId >> 8)
	act := uint8(msgId & 0xFF)
	return cmd, act
}
