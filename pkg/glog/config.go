package glog

import (
	"go.uber.org/zap/zapcore"
)

// Config glog 配置结构
type Config struct {
	// Path 日志文件路径
	Path string `json:"path" yaml:"path"`
	// Level 日志级别: debug, info, warn, error, dpanic, panic, fatal
	Level string `json:"level" yaml:"level"`
	// PrintConsole 是否同时输出到控制台
	PrintConsole bool `json:"printConsole" yaml:"printConsole"`
	// File 文件日志配置
	File FileConfig `json:"file" yaml:"file"`
}

// FileConfig 文件日志配置（lumberjack 配置）
type FileConfig struct {
	// MaxSize 单个日志文件最大大小（MB），超过则切割
	MaxSize int `json:"maxSize" yaml:"maxSize"`
	// MaxBackups 最大文件保留数，超过就删除最老的日志文件
	MaxBackups int `json:"maxBackups" yaml:"maxBackups"`
	// MaxAge 日志文件保留天数
	MaxAge int `json:"maxAge" yaml:"maxAge"`
	// Compress 是否压缩旧日志文件
	Compress bool `json:"compress" yaml:"compress"`
	// LocalTime 是否使用本地时间
	LocalTime bool `json:"localTime" yaml:"localTime"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Path:         "./logs/app.log",
		Level:        "info",
		PrintConsole: true,
		File: FileConfig{
			MaxSize:    500,
			MaxBackups: 100,
			MaxAge:     30,
			Compress:   false,
			LocalTime:  true,
		},
	}
}

// parseLevel 解析日志级别字符串为 zapcore.Level
func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "dpanic":
		return zapcore.DPanicLevel
	case "panic":
		return zapcore.PanicLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// InitFromConfig 根据配置初始化 glog
func InitFromConfig(cfg *Config) error {
	Init(cfg)
	return nil
}
