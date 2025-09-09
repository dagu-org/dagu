package otel

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// TracerName is the name of the tracer
	TracerName = "github.com/dagu-org/dagu"
)

// Tracer wraps OpenTelemetry tracer with DAG-specific configuration
type Tracer struct {
	tracer   trace.Tracer
	provider *sdktrace.TracerProvider
	config   *digraph.OTelConfig
}

// NewTracer creates a new OpenTelemetry tracer for a DAG
func NewTracer(ctx context.Context, dag *digraph.DAG) (*Tracer, error) {
	if dag.OTel == nil || !dag.OTel.Enabled {
		return &Tracer{tracer: otel.Tracer(TracerName)}, nil
	}

	cfg, err := cmdutil.EvalObject(ctx, *dag.OTel, map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate OTel config: %w", err)
	}

	exporter, err := createExporter(ctx, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTel exporter: %w", err)
	}

	res, err := createResource(ctx, dag)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTel resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(provider)

	return &Tracer{
		tracer:   otel.Tracer(TracerName),
		provider: provider,
		config:   dag.OTel,
	}, nil
}

// createExporter creates an OTLP exporter based on the endpoint
func createExporter(ctx context.Context, config *digraph.OTelConfig) (sdktrace.SpanExporter, error) {
	endpoint := config.Endpoint
	if endpoint == "" {
		return nil, fmt.Errorf("OTel endpoint is required")
	}

	// Determine if it's HTTP or gRPC based on the endpoint
	isHTTP := len(endpoint) > 4 && endpoint[len(endpoint)-4:] == "/v1/traces"

	if isHTTP {
		return createHTTPExporter(ctx, config)
	}
	return createGRPCExporter(ctx, config)
}

// createHTTPExporter creates an OTLP HTTP exporter
func createHTTPExporter(ctx context.Context, config *digraph.OTelConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(config.Endpoint),
		otlptracehttp.WithHeaders(config.Headers),
	}

	if config.Timeout > 0 {
		opts = append(opts, otlptracehttp.WithTimeout(config.Timeout))
	}

	if config.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	} else {
		opts = append(opts, otlptracehttp.WithTLSClientConfig(&tls.Config{
			MinVersion: tls.VersionTLS12,
		}))
	}

	client := otlptracehttp.NewClient(opts...)
	return otlptrace.New(ctx, client)
}

// createGRPCExporter creates an OTLP gRPC exporter
func createGRPCExporter(ctx context.Context, config *digraph.OTelConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(config.Endpoint),
		otlptracegrpc.WithHeaders(config.Headers),
	}

	if config.Timeout > 0 {
		opts = append(opts, otlptracegrpc.WithTimeout(config.Timeout))
	}

	if config.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
		opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
	} else {
		opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
		}))))
	}

	client := otlptracegrpc.NewClient(opts...)
	return otlptrace.New(ctx, client)
}

// createResource creates the OpenTelemetry resource for the DAG
func createResource(_ context.Context, dag *digraph.DAG) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName("dagu"),
	}

	// Add custom resource attributes from config
	if dag.OTel != nil && dag.OTel.Resource != nil {
		// Set DAG_NAME environment variable for expansion
		if dag.Name != "" {
			_ = os.Setenv("DAG_NAME", dag.Name)
		}

		for key, val := range dag.OTel.Resource {
			switch v := val.(type) {
			case string:
				// Expand environment variables in string values
				expanded := os.ExpandEnv(v)
				attrs = append(attrs, attribute.String(key, expanded))
			case int:
				attrs = append(attrs, attribute.Int(key, v))
			case int64:
				attrs = append(attrs, attribute.Int64(key, int64(v)))
			case float64:
				attrs = append(attrs, attribute.Float64(key, v))
			case bool:
				attrs = append(attrs, attribute.Bool(key, v))
			}
		}
	}

	return resource.NewWithAttributes(
		semconv.SchemaURL,
		attrs...,
	), nil
}

// Start starts a new span for the DAG execution
func (t *Tracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if t.tracer == nil {
		// Return a no-op span if tracer is not initialized
		return ctx, trace.SpanFromContext(ctx)
	}
	return t.tracer.Start(ctx, spanName, opts...)
}

// Shutdown shuts down the tracer provider
func (t *Tracer) Shutdown(ctx context.Context) error {
	if t.provider != nil {
		return t.provider.Shutdown(ctx)
	}
	return nil
}

// IsEnabled returns true if OTel is enabled
func (t *Tracer) IsEnabled() bool {
	return t.config != nil && t.config.Enabled
}
