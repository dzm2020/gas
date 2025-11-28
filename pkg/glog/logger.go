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
	Init()
}

// Init 初始化全局 logger
func Init(options ...Option) {
	opts := LoadOptions(options...)
	atomicLevel = zap.NewAtomicLevelAt(opts.Level)

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

	loggerWriter := opts.Writer
	cores := make([]zapcore.Core, 0, 2)
	cores = append(cores, zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), zapcore.AddSync(loggerWriter), atomicLevel))
	if opts.PrintConsole {
		cores = append(cores, zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)), atomicLevel))
	}
	mulCore := zapcore.NewTee(cores...)
	zapOpts := []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zap.DPanicLevel),
		zap.AddCallerSkip(1),
	}
	zapOpts = append(zapOpts, opts.ZapOption...)
	logger := zap.New(mulCore, zapOpts...)
	sugaredLogger := logger.Sugar()

	loggerValue.Store(logger)
	sugaredValue.Store(sugaredLogger)
}

// Stop 停止 logger，同步所有缓冲的日志
func Stop() {
	if logger := loggerValue.Load(); logger != nil {
		if l, ok := logger.(*zap.Logger); ok {
			_ = l.Sync()
		}
	}
	if sugared := sugaredValue.Load(); sugared != nil {
		if sl, ok := sugared.(*zap.SugaredLogger); ok {
			_ = sl.Sync()
		}
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
	if logger := loggerValue.Load(); logger != nil {
		if l, ok := logger.(*zap.Logger); ok {
			newLogger := l.WithOptions(opts...)
			loggerValue.Store(newLogger)
			sugaredValue.Store(newLogger.Sugar())
		}
	}
}

// Debug 输出 Debug 级别日志
func Debug(msg string, fields ...zap.Field) {
	if logger := loggerValue.Load(); logger != nil {
		if l, ok := logger.(*zap.Logger); ok {
			l.Debug(msg, fields...)
		}
	}
}

// Info 输出 Info 级别日志
func Info(msg string, fields ...zap.Field) {
	if logger := loggerValue.Load(); logger != nil {
		if l, ok := logger.(*zap.Logger); ok {
			l.Info(msg, fields...)
		}
	}
}

// Warn 输出 Warn 级别日志
func Warn(msg string, fields ...zap.Field) {
	if logger := loggerValue.Load(); logger != nil {
		if l, ok := logger.(*zap.Logger); ok {
			l.Warn(msg, fields...)
		}
	}
}

// Error 输出 Error 级别日志
func Error(msg string, fields ...zap.Field) {
	if logger := loggerValue.Load(); logger != nil {
		if l, ok := logger.(*zap.Logger); ok {
			l.Error(msg, fields...)
		}
	}
}

// Panic 输出 Panic 级别日志并触发 panic
func Panic(msg string, fields ...zap.Field) {
	if logger := loggerValue.Load(); logger != nil {
		if l, ok := logger.(*zap.Logger); ok {
			l.Panic(msg, fields...)
		}
	}
}

// Fatal 输出 Fatal 级别日志并退出程序
func Fatal(msg string, fields ...zap.Field) {
	if logger := loggerValue.Load(); logger != nil {
		if l, ok := logger.(*zap.Logger); ok {
			l.Fatal(msg, fields...)
		}
	}
}

// Debugf 使用格式化字符串输出 Debug 级别日志
func Debugf(template string, args ...interface{}) {
	if sugared := sugaredValue.Load(); sugared != nil {
		if sl, ok := sugared.(*zap.SugaredLogger); ok {
			sl.Debugf(template, args...)
		}
	}
}

// Infof 使用格式化字符串输出 Info 级别日志
func Infof(template string, args ...interface{}) {
	if sugared := sugaredValue.Load(); sugared != nil {
		if sl, ok := sugared.(*zap.SugaredLogger); ok {
			sl.Infof(template, args...)
		}
	}
}

// Warnf 使用格式化字符串输出 Warn 级别日志
func Warnf(template string, args ...interface{}) {
	if sugared := sugaredValue.Load(); sugared != nil {
		if sl, ok := sugared.(*zap.SugaredLogger); ok {
			sl.Warnf(template, args...)
		}
	}
}

// Errorf 使用格式化字符串输出 Error 级别日志
func Errorf(template string, args ...interface{}) {
	if sugared := sugaredValue.Load(); sugared != nil {
		if sl, ok := sugared.(*zap.SugaredLogger); ok {
			sl.Errorf(template, args...)
		}
	}
}

// DPanicf 使用格式化字符串输出 DPanic 级别日志
func DPanicf(template string, args ...interface{}) {
	if sugared := sugaredValue.Load(); sugared != nil {
		if sl, ok := sugared.(*zap.SugaredLogger); ok {
			sl.DPanicf(template, args...)
		}
	}
}

// Panicf 使用格式化字符串输出 Panic 级别日志并触发 panic
func Panicf(template string, args ...interface{}) {
	if sugared := sugaredValue.Load(); sugared != nil {
		if sl, ok := sugared.(*zap.SugaredLogger); ok {
			sl.Panicf(template, args...)
		}
	}
}

// Fatalf 使用格式化字符串输出 Fatal 级别日志并退出程序
func Fatalf(template string, args ...interface{}) {
	if sugared := sugaredValue.Load(); sugared != nil {
		if sl, ok := sugared.(*zap.SugaredLogger); ok {
			sl.Fatalf(template, args...)
		}
	}
}
