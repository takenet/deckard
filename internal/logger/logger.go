package logger

import (
	"context"
	"os"

	"github.com/spf13/viper"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/project"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func ConfigureLogger() {
	var logger *zap.Logger

	stackTrace := zap.AddStacktrace(zapcore.ErrorLevel)

	debug := viper.GetBool(config.DEBUG)
	logType := viper.GetString(config.LOG_TYPE)

	if logType != "json" {
		zapConfig := zap.NewDevelopmentConfig()

		if !debug {
			zapConfig.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
		}

		logger, _ = zapConfig.Build(stackTrace, zap.AddCaller())

	} else {
		zapConfig := zap.NewProductionConfig()

		if debug {
			zapConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		}

		zapConfig.EncoderConfig.MessageKey = "message"
		zapConfig.EncoderConfig.LevelKey = "severity"
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

		logger, _ = zapConfig.Build(stackTrace, zap.AddCaller())
	}

	logger = logger.With(zap.String("application_name", project.Name), zap.String("application_version", project.Version))

	hostname, _ := os.Hostname()
	if hostname == "" {
		logger = logger.With(zap.String("hostname", "undefined_host"))
	} else {
		logger = logger.With(zap.String("hostname", hostname))
	}

	zap.ReplaceGlobals(logger)
}

func S(ctx context.Context) *zap.SugaredLogger {
	spanContext := trace.SpanContextFromContext(ctx)

	if !spanContext.HasTraceID() {
		return zap.S()
	}

	return zap.S().With(
		"trace_id", spanContext.TraceID().String(),
		"span_id", spanContext.SpanID().String(),
		"trace_flags", spanContext.TraceFlags().String(),
	)
}

func L(ctx context.Context) *zap.Logger {
	spanContext := trace.SpanContextFromContext(ctx)

	if !spanContext.HasTraceID() {
		return zap.L()
	}

	return zap.L().With(
		zap.String("trace_id", spanContext.TraceID().String()),
		zap.String("span_id", spanContext.SpanID().String()),
		zap.String("trace_flags", spanContext.TraceFlags().String()),
	)
}
