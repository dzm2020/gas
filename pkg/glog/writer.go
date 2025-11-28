package glog

import (
	"io"

	"gopkg.in/natefinch/lumberjack.v2"
)

func defaultWriter(filename string) io.Writer {
	return newWriter(filename, FileConfig{
		MaxSize:    500,
		MaxBackups: 100,
		MaxAge:     30,
		LocalTime:  true,
		Compress:   false,
	})
}

// newWriter 根据配置创建日志文件写入器
func newWriter(filename string, fileConfig FileConfig) io.Writer {
	cfg := fileConfig
	// 应用默认值
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 500
	}
	if cfg.MaxBackups == 0 {
		cfg.MaxBackups = 100
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 30
	}

	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		LocalTime:  cfg.LocalTime,
		Compress:   cfg.Compress,
	}
}
