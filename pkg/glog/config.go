package glog

import (
	"fmt"
	"gas/pkg/lib"
	"os"

	"go.uber.org/zap/zapcore"
)

// Config glog 配置结构
type Config struct {
	// Path 日志文件路径
	Path string `json:"path"`
	// Level 日志级别: debug, info, warn, error, dpanic, panic, fatal
	Level string `json:"level"`
	// PrintConsole 是否同时输出到控制台
	PrintConsole bool `json:"printConsole"`
	// File 文件日志配置
	File FileConfig `json:"file"`
}

// FileConfig 文件日志配置（lumberjack 配置）
type FileConfig struct {
	// MaxSize 单个日志文件最大大小（MB），超过则切割
	MaxSize int `json:"maxSize"`
	// MaxBackups 最大文件保留数，超过就删除最老的日志文件
	MaxBackups int `json:"maxBackups"`
	// MaxAge 日志文件保留天数
	MaxAge int `json:"maxAge"`
	// Compress 是否压缩旧日志文件
	Compress bool `json:"compress"`
	// LocalTime 是否使用本地时间
	LocalTime bool `json:"localTime"`
}

// LoadConfig 从配置文件加载配置
func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config file failed: %w", err)
	}

	var cfg Config
	if err := lib.Json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %w", err)
	}

	return &cfg, nil
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
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 应用默认值
	if cfg.Path == "" {
		cfg.Path = "./logs/app.log"
	}
	if cfg.Level == "" {
		cfg.Level = "info"
	}
	if cfg.File.MaxSize == 0 {
		cfg.File.MaxSize = 500
	}
	if cfg.File.MaxBackups == 0 {
		cfg.File.MaxBackups = 100
	}
	if cfg.File.MaxAge == 0 {
		cfg.File.MaxAge = 30
	}

	// 构建 options（注意顺序：先设置 FileConfig，再设置 Path，这样 Path 可以使用配置）
	opts := []Option{
		WithLevel(parseLevel(cfg.Level)),
		WithPrintConsole(cfg.PrintConsole),
		WithFileConfig(cfg.File),
		WithPath(cfg.Path),
	}

	Init(opts...)
	return nil
}

// InitFromFile 从配置文件路径初始化 glog
func InitFromFile(configPath string) error {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}
	return InitFromConfig(cfg)
}
