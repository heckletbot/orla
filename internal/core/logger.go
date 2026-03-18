package core

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitLogger initializes zap's global logger.
// pretty: use development (pretty) format when true, production JSON when false.
// level: "debug", "info", "warn", "error", "fatal"; empty means "info".
// Output is stderr to avoid interfering with tool stdout (critical for stdio/MCP).
func InitLogger(pretty bool, level string) error {
	var config zap.Config

	if pretty {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	if level != "" {
		var l zapcore.Level
		if err := l.UnmarshalText([]byte(level)); err != nil {
			return fmt.Errorf("invalid log level %q: %w", level, err)
		}
		config.Level = zap.NewAtomicLevelAt(l)
	}

	config.OutputPaths = []string{"stderr"}
	config.ErrorOutputPaths = []string{"stderr"}

	logger, err := config.Build()
	if err != nil {
		return fmt.Errorf("failed to build logger: %w", err)
	}

	zap.ReplaceGlobals(logger)
	return nil
}

// LogDeferredError takes a function that returns an error, calls it, and logs the error if it is not nil
func LogDeferredError(fn func() error) {
	if err := fn(); err != nil {
		zap.L().Error("Deferred error", zap.Error(err), zap.Stack("stack_trace"))
	}
}

// LogDeferredError1 takes a function that returns an error, calls it with the given argument, and logs the error if it is not nil
func LogDeferredError1[T any](fn func(T) error, arg T) {
	if err := fn(arg); err != nil {
		zap.L().Error("Deferred error", zap.Error(err), zap.Stack("stack_trace"))
	}
}
