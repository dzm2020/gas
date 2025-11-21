package consul

import "time"

type Option func(*Options)

type Options struct {
	Address            string
	WatchWaitTime      time.Duration
	HealthTTL          time.Duration
	DeregisterInterval time.Duration
}

func defaultOptions() *Options {
	return &Options{
		Address:            "127.0.0.1:8500",
		WatchWaitTime:      1 * time.Second,
		HealthTTL:          1 * time.Second,
		DeregisterInterval: 3 * time.Second,
	}
}

func loadOptions(opts ...Option) *Options {
	options := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(options)
		}
	}
	return options
}

func WithAddress(addr string) Option {
	return func(o *Options) {
		o.Address = addr
	}
}

func WithWatchWaitTime(d time.Duration) Option {
	return func(o *Options) {
		o.WatchWaitTime = d
	}
}

func WithHealthTTL(d time.Duration) Option {
	return func(o *Options) {
		o.HealthTTL = d
	}
}

func WithDeregisterInterval(d time.Duration) Option {
	return func(o *Options) {
		o.DeregisterInterval = d
	}
}
