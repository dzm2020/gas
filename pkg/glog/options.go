// Package glog provides a structured logging wrapper around zap.
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
	fileConfig   *FileConfig // 保存文件配置，用于延迟创建 writer
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

// WithPath 设置日志文件路径
func WithPath(path string) Option {
	return func(opts *Options) {
		opts.Path = path
		// 如果已有文件配置，使用配置创建 writer；否则使用默认配置
		if opts.fileConfig != nil {
			opts.Writer = newWriter(path, *opts.fileConfig)
		} else {
			opts.Writer = defaultWriter(path)
		}
	}
}

// WithLevel 设置日志级别
func WithLevel(level zapcore.Level) Option {
	return func(opts *Options) {
		opts.Level = level
	}
}

// WithPrintConsole 设置是否输出到控制台
func WithPrintConsole(printConsole bool) Option {
	return func(opts *Options) {
		opts.PrintConsole = printConsole
	}
}

// WithFileConfig 设置文件日志配置
func WithFileConfig(fileConfig FileConfig) Option {
	return func(opts *Options) {
		// 保存文件配置
		opts.fileConfig = &fileConfig
		// 如果已有路径，立即创建 writer
		if opts.Path != "" {
			opts.Writer = newWriter(opts.Path, fileConfig)
		}
	}
}

// WithZapOption 添加 zap.Option
func WithZapOption(zapOpts ...zap.Option) Option {
	return func(opts *Options) {
		opts.ZapOption = append(opts.ZapOption, zapOpts...)
	}
}
