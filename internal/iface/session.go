package iface

import (
	"gas/internal/gate/protocol"
	"gas/pkg/network"
)

func (m *Session) Response(ctx IContext, message *Message) error {
	connection := network.GetConnection(m.GetEntityId())
	if connection == nil {
		return nil
	}
	
	if ctx.ID() == m.GetAgent() {
		//  构造协议消息
		cmd, act := protocol.ParseId(uint16(message.GetId()))
		protocolMsg := protocol.New(cmd, act, message.GetData())
		protocolMsg.Index = m.GetIndex()
		return connection.Send(protocolMsg)
	} else {
		return ctx.System().Send(message)
	}
}

//func (m *Session) ResponseCode(system iface.ISystem, code int64) {
//	system.Send(m.GetAgent(), todo)
//}
