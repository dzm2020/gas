package iface

import "gas/pkg/network"

func (m *Session) Response(ctx IContext, message *Message) error {
	
	connection := network.GetConnection(m.GetEntityId())
	if ctx.ID() == m.GetAgent() {
		//  构造消息
		return connection.Send()
	} else {
		return system.Send(message)
	}
}

//func (m *Session) ResponseCode(system iface.ISystem, code int64) {
//	system.Send(m.GetAgent(), todo)
//}
