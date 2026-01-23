package network

import (
	"context"
	"fmt"
	"sync"

	"github.com/dzm2020/gas/pkg/lib/grs"
	"github.com/dzm2020/gas/pkg/lib/stopper"
)

func newBaseServer(network, address string, handler IHandler, option ...Option) *baseServer {
	server := &baseServer{
		options:      loadOptions(option...),
		network:      network,
		address:      address,
		protoAddress: fmt.Sprintf("%s:%s", network, address),
		handler:      handler,
	}
	server.ctx, server.cancel = context.WithCancel(context.Background())
	return server
}

type baseServer struct {
	stopper.Stopper
	options          *Options
	handler          IHandler
	network, address string // 监听地址（如 ":8080"）
	protoAddress     string
	waitGroup        sync.WaitGroup
	ctx              context.Context
	cancel           context.CancelFunc
}

func (s *baseServer) Addr() string {
	return s.protoAddress
}

func (s *baseServer) Shutdown(ctx context.Context) {
	s.cancel()
	grs.GroupWaitWithContext(ctx, &s.waitGroup)
	return
}
