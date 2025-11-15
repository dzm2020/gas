/**
 * @Author: dingQingHui
 * @Description:
 * @File: protocol
 * @Version: 1.0.0
 * @Date: 2024/12/6 15:52
 */

package protocol

const HeadLen = 13

const (
	CmdSystem uint8 = 0
)

const (
	_            uint8 = iota
	ActHandshake       // 握手   用于交换密钥 版本等信息
	ActHeartbeat       // 心跳
	ActKick            // 强制关闭连接
)

func New() *Message {
	return &Message{
		Head: new(Head),
		Data: nil,
	}
}

func NewWithData(data []byte) *Message {
	return &Message{
		Head: new(Head),
		Data: data,
	}
}

func NewErr(code uint16) *Message {
	m := New()
	m.Error = code
	return m
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
	Act   uint8  // 命令
	Error uint16 // 错误码
	Index uint32 // 序号
}

func CmdAct(cmd, act uint8) uint16 {
	return uint16(cmd)<<8 + uint16(act)
}
