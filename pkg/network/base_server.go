package network

import (
	"context"
	"fmt"
	"github.com/dzm2020/gas/pkg/lib/stopper"
	"sync"
)

func newBaseServer(ctx context.Context, network, address string, option ...Option) *baseServer {
	server := &baseServer{
		options:      loadOptions(option...),
		network:      network,
		address:      address,
		protoAddress: fmt.Sprintf("%s:%s", network, address),
	}
	server.ctx, server.cancel = context.WithCancel(ctx)
	return server
}

type baseServer struct {
	stopper.Stopper
	options          *Options
	network, address string // 监听地址（如 ":8080"）
	protoAddress     string
	waitGroup        sync.WaitGroup
	ctx              context.Context
	cancel           context.CancelFunc
}

func (s *baseServer) Addr() string {
	return s.protoAddress
}
