package network

import (
	"context"

	"github.com/panjf2000/gnet/v2"
)

func newBaseServer(opts *Options, protoAddr string) *baseServer {
	m := &baseServer{
		opts:      opts,
		protoAddr: protoAddr,
	}
	return m
}

type baseServer struct {
	gnet.BuiltinEventEngine
	opts      *Options
	protoAddr string
	eng       gnet.Engine
}

func (m *baseServer) OnBoot(eng gnet.Engine) (action gnet.Action) {
	m.eng = eng
	return gnet.None
}

func (m *baseServer) Stop() error {
	return m.eng.Stop(context.Background())
}

func (m *baseServer) Options() *Options {
	return m.opts
}
