package trace

import (
	"context"
	"os"

	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/project"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

var tracerProvider *sdktrace.TracerProvider

func Init() error {
	ctx := context.Background()

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		logger.S(ctx).Warn("Disabling tracing because OpenTelemetry exporter environment variables are not set.")

		return nil
	}

	client := otlptracegrpc.NewClient()
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(newResource()),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return nil
}

func newResource() *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(project.Name),
		semconv.ServiceVersionKey.String(project.Version),
	)
}

func Shutdown() {
	if tracerProvider == nil {
		return
	}

	if err := tracerProvider.Shutdown(context.Background()); err != nil {
		logger.S(context.Background()).Error("Error shutting down tracer provider.", err)
	} else {
		tracerProvider = nil
	}
}
