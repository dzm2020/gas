package glog

import (
	"os"
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	loggerValue  atomic.Value // *zap.Logger
	sugaredValue atomic.Value // *zap.SugaredLogger
	atomicLevel  zap.AtomicLevel
)

func init() {
	Init(DefaultConfig())
}

// Init 初始化全局 logger
// cfg: 配置对象，如果为 nil 则使用默认配置
func Init(cfg *Config) {
	if cfg == nil {
		return
	}
	atomicLevel = zap.NewAtomicLevelAt(parseLevel(cfg.Level))
	encoderConfig := zapcore.EncoderConfig{
		MessageKey:     "M",
		LevelKey:       "L",
		TimeKey:        "T",
		CallerKey:      "C",
		NameKey:        "N",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006/01/02 15:04:05.000000Z0700"),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 创建文件写入器
	loggerWriter := newWriter(cfg.Path, cfg.File)

	cores := make([]zapcore.Core, 0, 2)
	cores = append(cores, zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), zapcore.AddSync(loggerWriter), atomicLevel))
	if cfg.PrintConsole {
		cores = append(cores, zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)), atomicLevel))
	}
	mulCore := zapcore.NewTee(cores...)
	zapOpts := []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zap.ErrorLevel),
		zap.AddCallerSkip(1),
	}
	logger := zap.New(mulCore, zapOpts...)
	sugaredLogger := logger.Sugar()

	loggerValue.Store(logger)
	sugaredValue.Store(sugaredLogger)
}

// Stop 停止 logger，同步所有缓冲的日志
func Stop() {
	if l := getLogger(); l != nil {
		_ = l.Sync()
	}
	if sl := getSugaredLogger(); sl != nil {
		_ = sl.Sync()
	}
}

// SetLogLevel 设置日志级别
func SetLogLevel(logLevel zapcore.Level) {
	atomicLevel.SetLevel(logLevel)
}

// GetLevel 获取当前日志级别
func GetLevel() zapcore.Level {
	return atomicLevel.Level()
}

// WithOptions 添加 zap.Option
func WithOptions(opts ...zap.Option) {
	if l := getLogger(); l != nil {
		newLogger := l.WithOptions(opts...)
		loggerValue.Store(newLogger)
		sugaredValue.Store(newLogger.Sugar())
	}
}

// getLogger 获取当前 logger
func getLogger() *zap.Logger {
	if v := loggerValue.Load(); v != nil {
		if l, ok := v.(*zap.Logger); ok {
			return l
		}
	}
	return nil
}

// getSugaredLogger 获取当前 sugared logger
func getSugaredLogger() *zap.SugaredLogger {
	if v := sugaredValue.Load(); v != nil {
		if sl, ok := v.(*zap.SugaredLogger); ok {
			return sl
		}
	}
	return nil
}

// Debug 输出 Debug 级别日志
func Debug(msg string, fields ...zap.Field) {
	if l := getLogger(); l != nil {
		l.Debug(msg, fields...)
	}
}

// Info 输出 Info 级别日志
func Info(msg string, fields ...zap.Field) {
	if l := getLogger(); l != nil {
		l.Info(msg, fields...)
	}
}

// Warn 输出 Warn 级别日志
func Warn(msg string, fields ...zap.Field) {
	if l := getLogger(); l != nil {
		l.Warn(msg, fields...)
	}
}

// Error 输出 Error 级别日志
func Error(msg string, fields ...zap.Field) {
	if l := getLogger(); l != nil {
		l.Error(msg, fields...)
	}
}

// Panic 输出 Panic 级别日志并触发 panic
func Panic(msg string, fields ...zap.Field) {
	if l := getLogger(); l != nil {
		l.Panic(msg, fields...)
	}
}

// Fatal 输出 Fatal 级别日志并退出程序
func Fatal(msg string, fields ...zap.Field) {
	if l := getLogger(); l != nil {
		l.Fatal(msg, fields...)
	}
}

// Debugf 使用格式化字符串输出 Debug 级别日志
func Debugf(template string, args ...interface{}) {
	if sl := getSugaredLogger(); sl != nil {
		sl.Debugf(template, args...)
	}
}

// Infof 使用格式化字符串输出 Info 级别日志
func Infof(template string, args ...interface{}) {
	if sl := getSugaredLogger(); sl != nil {
		sl.Infof(template, args...)
	}
}

// Warnf 使用格式化字符串输出 Warn 级别日志
func Warnf(template string, args ...interface{}) {
	if sl := getSugaredLogger(); sl != nil {
		sl.Warnf(template, args...)
	}
}

// Errorf 使用格式化字符串输出 Error 级别日志
func Errorf(template string, args ...interface{}) {
	if sl := getSugaredLogger(); sl != nil {
		sl.Errorf(template, args...)
	}
}

// DPanicf 使用格式化字符串输出 DPanic 级别日志
func DPanicf(template string, args ...interface{}) {
	if sl := getSugaredLogger(); sl != nil {
		sl.DPanicf(template, args...)
	}
}

// Panicf 使用格式化字符串输出 Panic 级别日志并触发 panic
func Panicf(template string, args ...interface{}) {
	if sl := getSugaredLogger(); sl != nil {
		sl.Panicf(template, args...)
	}
}

// Fatalf 使用格式化字符串输出 Fatal 级别日志并退出程序
func Fatalf(template string, args ...interface{}) {
	if sl := getSugaredLogger(); sl != nil {
		sl.Fatalf(template, args...)
	}
}
