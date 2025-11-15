/**
 * @Author: dingQingHui
 * @Description:
 * @File: config
 * @Version: 1.0.0
 * @Date: 2024/9/23 11:14
 */

package glog

import (
	"io"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Option func(*Options)

type Options struct {
	path         string
	level        zapcore.Level
	printConsole bool
	zapOption    []zap.Option
	writer       io.Writer
}

func loadOptions(options ...Option) *Options {
	opts := defaultOptions()
	for _, option := range options {
		option(opts)
	}
	return opts
}

func defaultOptions() *Options {
	const defaultPath = "./logs/app.log"
	return &Options{
		path:         defaultPath,
		level:        zapcore.InfoLevel,
		printConsole: true,
		zapOption:    nil,
		writer:       defaultWriter(defaultPath),
	}
}
