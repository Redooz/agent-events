package logger

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"agent-events/server/internal/core/port"
	"agent-events/server/pkg/config"
)

type Adapter struct {
	zap *zap.Logger
}

func New(z *zap.Logger) port.Logger {
	return &Adapter{zap: z}
}

func NewZapLogger(cfg config.Config) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid LOG_LEVEL %q: %w", cfg.LogLevel, err)
	}

	zapCfg := zap.NewProductionConfig()
	if !cfg.IsProduction() {
		zapCfg = zap.NewDevelopmentConfig()
	}

	zapCfg.Level = zap.NewAtomicLevelAt(level)

	z, err := zapCfg.Build(zap.AddStacktrace(zapcore.ErrorLevel))
	if err != nil {
		return nil, fmt.Errorf("build zap logger: %w", err)
	}

	return z, nil
}

func (a *Adapter) Debug(msg string, fields ...port.Field) {
	a.zap.Debug(msg, zapFields(fields)...)
}

func (a *Adapter) Info(msg string, fields ...port.Field) {
	a.zap.Info(msg, zapFields(fields)...)
}

func (a *Adapter) Warn(msg string, fields ...port.Field) {
	a.zap.Warn(msg, zapFields(fields)...)
}

func (a *Adapter) Error(msg string, fields ...port.Field) {
	a.zap.Error(msg, zapFields(fields)...)
}

func zapFields(fields []port.Field) []zap.Field {
	out := make([]zap.Field, 0, len(fields))
	for _, f := range fields {
		out = append(out, zapField(f))
	}

	return out
}

func zapField(f port.Field) zap.Field {
	if f.Value == nil {
		return zap.Any(f.Key, nil)
	}

	switch value := f.Value.(type) {
	case error:
		return zap.NamedError(f.Key, value)
	case string:
		return zap.String(f.Key, value)
	case int:
		return zap.Int(f.Key, value)
	case int64:
		return zap.Int64(f.Key, value)
	case bool:
		return zap.Bool(f.Key, value)
	case time.Duration:
		return zap.Duration(f.Key, value)
	case time.Time:
		return zap.Time(f.Key, value)
	case fmt.Stringer:
		return zap.Stringer(f.Key, value)
	default:
		return zap.Any(f.Key, f.Value)
	}
}
