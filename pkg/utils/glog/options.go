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
	Path         string
	Level        zapcore.Level
	PrintConsole bool
	ZapOption    []zap.Option
	Writer       io.Writer
}

func LoadOptions(options ...Option) *Options {
	opts := defaultOptions()
	for _, option := range options {
		option(opts)
	}
	return opts
}

func defaultOptions() *Options {
	const defaultPath = "./logs/app.log"
	return &Options{
		Path:         defaultPath,
		Level:        zapcore.InfoLevel,
		PrintConsole: true,
		ZapOption:    nil,
		Writer:       defaultWriter(defaultPath),
	}
}
