package logger

import (
	"io"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/TiaraBasori/PaperValet/internal/interfaces"
)

var (
	globalLogger *zap.Logger
	globalLevel  zap.AtomicLevel
	mu           sync.RWMutex
)

// Init initializes the global logger.
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

// Get returns the global logger.
func Get() *zap.Logger {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger == nil {
		l, _ := zap.NewDevelopment()
		return l
	}
	return globalLogger
}

// Sync flushes the global logger.
func Sync() error {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// SetOutput sets the output writer for the global logger.
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

// ZapLogger adapts *zap.Logger to interfaces.Logger.
type ZapLogger struct {
	logger *zap.Logger
}

func (z *ZapLogger) Debug(msg string, keysAndValues ...any) {
	z.logger.Debug(msg, kvToFields(keysAndValues)...)
}

func (z *ZapLogger) Info(msg string, keysAndValues ...any) {
	z.logger.Info(msg, kvToFields(keysAndValues)...)
}

func (z *ZapLogger) Warn(msg string, keysAndValues ...any) {
	z.logger.Warn(msg, kvToFields(keysAndValues)...)
}

func (z *ZapLogger) Error(msg string, keysAndValues ...any) {
	z.logger.Error(msg, kvToFields(keysAndValues)...)
}

func (z *ZapLogger) With(keysAndValues ...any) interfaces.Logger {
	return &ZapLogger{logger: z.logger.With(kvToFields(keysAndValues)...)}
}

func (z *ZapLogger) Named(name string) interfaces.Logger {
	return &ZapLogger{logger: z.logger.Named(name)}
}

func kvToFields(kv []any) []zap.Field {
	if len(kv) == 0 {
		return nil
	}
	fields := make([]zap.Field, 0, len(kv)/2+1)
	for i := 0; i < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			continue
		}
		var val any
		if i+1 < len(kv) {
			val = kv[i+1]
		}
		fields = append(fields, zap.Any(key, val))
	}
	return fields
}

// GlobalLogger returns an interfaces.Logger backed by the global zap logger.
func GlobalLogger() interfaces.Logger {
	return &ZapLogger{logger: Get()}
}

// NamedLogger creates a named interfaces.Logger from the global logger.
func NamedLogger(name string) interfaces.Logger {
	return &ZapLogger{logger: Get().Named(name)}
}

// StdLogWriter returns an io.Writer that logs to the global logger.
func StdLogWriter() io.Writer {
	return &stdLogWriter{}
}

type stdLogWriter struct{}

func (w *stdLogWriter) Write(p []byte) (n int, err error) {
	Get().Info(strings.TrimSpace(string(p)))
	return len(p), nil
}