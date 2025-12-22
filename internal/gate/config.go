package gate

import (
	"time"

	"gas/pkg/network"
)

func ToOptions(c *Config) []network.Option {
	var options []network.Option
	if c.KeepAlive > 0 {
		options = append(options, network.WithKeepAlive(time.Duration(c.KeepAlive)*time.Second))
	}
	if c.SendChanSize > 0 {
		options = append(options, network.WithSendChanSize(c.SendChanSize))
	}
	if c.ReadBufSize > 0 {
		options = append(options, network.WithReadBufSize(c.ReadBufSize))
	}
	if c.MaxConn > 0 {
		options = append(options, network.WithMaxConn(c.MaxConn))
	}
	return options
}
