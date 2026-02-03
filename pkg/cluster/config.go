package cluster

import (
	dis "github.com/dzm2020/gas/pkg/discovery"
	mq "github.com/dzm2020/gas/pkg/messageQue"
)

type Config struct {
	Name         string      `json:"name" yaml:"name"`
	Discovery    *dis.Config `json:"discovery" yaml:"discovery"`
	MessageQueue *mq.Config  `json:"messageQueue" yaml:"messageQueue"`
}

func DefaultConfig() *Config {
	return &Config{
		Name: "",
		Discovery: &dis.Config{
			Type:   "consul",
			Config: nil,
		},

		MessageQueue: &mq.Config{
			Type:   "nats",
			Config: nil,
		},
	}
}
