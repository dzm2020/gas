package glog

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *logger

func init() {
	log = newLogger()
}

func newLogger(options ...Option) *logger {
	opts := loadOptions(options...)
	l := new(logger)
	l.options = opts
	l.Init()
	return l
}

type logger struct {
	*zap.Logger
	*zap.SugaredLogger
	options  *Options
	loglevel zap.AtomicLevel
}

func (l *logger) Init() {
	level := l.options.level
	l.loglevel = zap.NewAtomicLevelAt(level)
	encoderConfig := zapcore.EncoderConfig{
		MessageKey:     "M",                                                            // 结构化（json）输出：msg的key
		LevelKey:       "L",                                                            // 结构化（json）输出：日志级别的key（INFO，WARN，ERROR等）
		TimeKey:        "T",                                                            // 结构化（json）输出：时间的key
		CallerKey:      "C",                                                            // 结构化（json）输出：打印日志的文件对应的Key
		NameKey:        "N",                                                            // 结构化（json）输出: 日志名
		StacktraceKey:  "S",                                                            // 结构化（json）输出: 堆栈
		LineEnding:     zapcore.DefaultLineEnding,                                      // 换行符
		EncodeLevel:    zapcore.LowercaseLevelEncoder,                                  // 将日志级别转换成大写（INFO，WARN，ERROR等）
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006/01/02 15:04:05.000000Z0700"), // 日志时间的输出样式
		EncodeDuration: zapcore.SecondsDurationEncoder,                                 // 消耗时间的输出样式
		EncodeCaller:   zapcore.ShortCallerEncoder,                                     // 采用短文件路径编码输出（test/main.go:14 ）
	}

	// 获取io.Writer的实现
	loggerWriter := l.options.writer
	// 实现多个输出
	var cores []zapcore.Core
	cores = append(cores, zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), zapcore.AddSync(loggerWriter), level))
	if l.options.printConsole {
		// 同时将日志输出到控制台，NewJSONEncoder 是结构化输出
		cores = append(cores, zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)), level))
	}
	mulCore := zapcore.NewTee(cores...)
	// 设置初始化字段
	var options = []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zap.DPanicLevel),
		zap.AddCallerSkip(1),
	}
	options = append(options, l.options.zapOption...)
	l.Logger = zap.New(mulCore, options...)
	l.SugaredLogger = l.Logger.Sugar()
}

func (l *logger) Name() string {
	return "logger"
}
func (l *logger) Stop() {
	_ = l.Logger.Sync()
	_ = l.SugaredLogger.Sync()
}

func Init(options ...Option) {
	log = newLogger(options...)
}

func SetLogLevel(logLevel zapcore.Level) {
	if log == nil {
		return
	}
	log.loglevel.SetLevel(logLevel)
}
func Stop(options ...Option) {
	log.Stop()
}

func GetLevel() zapcore.Level {
	if log == nil {
		return zapcore.DebugLevel
	}
	return log.loglevel.Level()
}

func Debug(msg string, fields ...zap.Field) {
	log.Logger.Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	log.Logger.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	log.Logger.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	log.Logger.Error(msg, fields...)
}

func Panic(msg string, fields ...zap.Field) {
	log.Logger.DPanic(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	log.Logger.Fatal(msg, fields...)
}

func Debugf(template string, args ...interface{}) {
	log.SugaredLogger.Debugf(template, args...)
}

func Infof(template string, args ...interface{}) {
	log.SugaredLogger.Infof(template, args...)
}

func Warnf(template string, args ...interface{}) {
	log.SugaredLogger.Warnf(template, args...)
}

func Errorf(template string, args ...interface{}) {
	log.SugaredLogger.Errorf(template, args...)
}

func DPanicf(template string, args ...interface{}) {
	log.SugaredLogger.DPanicf(template, args...)
}

func Panicf(template string, args ...interface{}) {
	log.SugaredLogger.Panicf(template, args...)
}

func Fatalf(template string, args ...interface{}) {
	log.SugaredLogger.Fatalf(template, args...)
}
