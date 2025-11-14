/**
 * @Author: dingQingHui
 * @Description:
 * @File: server
 * @Version: 1.0.0
 * @Date: 2024/11/29 14:49
 */

package network

import (
	"gas/pkg/utils/netx"
	"gas/pkg/utils/xerror"
)

func NewListener(protoAddr string, options ...Option) IListener {
	s := newService(protoAddr, options...)
	return s
}

func newService(protoAddr string, options ...Option) IListener {
	proto, _, err := netx.ParseProtoAddr(protoAddr)
	xerror.Assert(err)
	opts := loadOptions(options...)
	switch proto {
	case "udp", "udp4", "udp6":
		return newUdpServer(opts, protoAddr)
	case "tcp", "tcp4", "tcp6":
		return newTcpServer(opts, protoAddr)
	}
	return nil
}
