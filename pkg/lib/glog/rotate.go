package glog

import (
	"io"

	"gopkg.in/natefinch/lumberjack.v2"
)

// newWriter 根据配置创建日志文件写入器
func newWriter(filename string, fileConfig FileConfig) io.Writer {
	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    fileConfig.MaxSize,
		MaxBackups: fileConfig.MaxBackups,
		MaxAge:     fileConfig.MaxAge,
		LocalTime:  fileConfig.LocalTime,
		Compress:   fileConfig.Compress,
	}
}
