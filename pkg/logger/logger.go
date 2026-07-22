package logger

import (
	"io"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.Logger
	globalLevel  zap.AtomicLevel
	mu           sync.RWMutex
)

func Init(level, format string) error {
	var cfg zap.Config
	if format == "json" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	var zapLevel zapcore.Level
	switch strings.ToUpper(level) {
	case "DEBUG":
		zapLevel = zapcore.DebugLevel
	case "INFO":
		zapLevel = zapcore.InfoLevel
	case "WARN":
		zapLevel = zapcore.WarnLevel
	case "ERROR":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	cfg.Level = zap.NewAtomicLevelAt(zapLevel)
	logger, err := cfg.Build()
	if err != nil {
		return err
	}

	mu.Lock()
	globalLogger = logger
	globalLevel = cfg.Level
	mu.Unlock()
	return nil
}

func Get() *zap.Logger {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger == nil {
		l, _ := zap.NewDevelopment()
		return l
	}
	return globalLogger
}

func Sync() error {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

func SetOutput(w io.Writer) error {
	mu.Lock()
	defer mu.Unlock()
	if globalLogger != nil {
		level := globalLevel
		globalLogger = globalLogger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewCore(
				zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
				zapcore.AddSync(w),
				level,
			)
		}))
	}
	return nil
}

// Convenience functions
func Debug(msg string, fields ...zap.Field)   { Get().Debug(msg, fields...) }
func Info(msg string, fields ...zap.Field)    { Get().Info(msg, fields...) }
func Warn(msg string, fields ...zap.Field)    { Get().Warn(msg, fields...) }
func Error(msg string, fields ...zap.Field)   { Get().Error(msg, fields...) }
func Fatal(msg string, fields ...zap.Field)   { Get().Fatal(msg, fields...) }

func Debugf(format string, args ...interface{}) { Get().Debug(msgf(format, args...)) }
func Infof(format string, args ...interface{})  { Get().Info(msgf(format, args...)) }
func Warnf(format string, args ...interface{})  { Get().Warn(msgf(format, args...)) }
func Errorf(format string, args ...interface{}) { Get().Error(msgf(format, args...)) }
func Fatalf(format string, args ...interface{}) { Get().Fatal(msgf(format, args...)) }

func msgf(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	return format
}

func With(fields ...zap.Field) *zap.Logger       { return Get().With(fields...) }
func Named(name string) *zap.Logger              { return Get().Named(name) }
func Level() zapcore.Level                       { return globalLevel.Level() }
func StringToZapLevel(level string) zapcore.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return zapcore.DebugLevel
	case "INFO":
		return zapcore.InfoLevel
	case "WARN":
		return zapcore.WarnLevel
	case "ERROR":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func StdLogWriter() io.Writer {
	return &stdLogWriter{}
}

type stdLogWriter struct{}

func (w *stdLogWriter) Write(p []byte) (n int, err error) {
	Info(strings.TrimSpace(string(p)))
	return len(p), nil
}