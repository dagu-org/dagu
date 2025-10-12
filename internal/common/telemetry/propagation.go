package telemetry

import (
	"context"
	"os"

	"github.com/dagu-org/dagu/internal/common/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TraceContextCarrier is a carrier for W3C trace context propagation via environment variables
type TraceContextCarrier struct {
	env map[string]string
}

// NewTraceContextCarrier creates a new carrier for trace context propagation
func NewTraceContextCarrier() *TraceContextCarrier {
	return &TraceContextCarrier{
		env: make(map[string]string),
	}
}

// Get returns the value associated with the passed key
func (c *TraceContextCarrier) Get(key string) string {
	// Check with uppercase for environment variables
	switch key {
	case "traceparent":
		return c.env["TRACEPARENT"]
	case "tracestate":
		return c.env["TRACESTATE"]
	}
	return c.env[key]
}

// Set stores the key-value pair
func (c *TraceContextCarrier) Set(key string, value string) {
	// Convert to uppercase for environment variables
	switch key {
	case "traceparent":
		c.env["TRACEPARENT"] = value
	case "tracestate":
		c.env["TRACESTATE"] = value
	default:
		c.env[key] = value
	}
}

// Keys lists the keys stored in this carrier
func (c *TraceContextCarrier) Keys() []string {
	keys := make([]string, 0, len(c.env))
	for k := range c.env {
		keys = append(keys, k)
	}
	return keys
}

// ToEnv converts the carrier to environment variables format
func (c *TraceContextCarrier) ToEnv() []string {
	env := make([]string, 0, len(c.env))
	for k, v := range c.env {
		env = append(env, k+"="+v)
	}
	return env
}

// InitializePropagators sets up W3C trace context propagator as the global propagator
func InitializePropagators() {
	// Set up W3C Trace Context propagator
	tc := propagation.TraceContext{}
	// Set it as the global propagator
	otel.SetTextMapPropagator(tc)
}

// InjectTraceContext injects the trace context from the current span into environment variables
func InjectTraceContext(ctx context.Context) []string {
	// Check if we have an active span
	span := trace.SpanFromContext(ctx)
	spanCtx := span.SpanContext()

	logger.Debug(ctx, "InjectTraceContext called",
		"hasActiveSpan", spanCtx.IsValid(),
		"traceID", spanCtx.TraceID().String(),
		"spanID", spanCtx.SpanID().String(),
		"traceFlags", spanCtx.TraceFlags(),
	)

	// Get the global propagator
	prop := otel.GetTextMapPropagator()

	// Create a carrier for the trace context
	carrier := NewTraceContextCarrier()

	// Inject the trace context into the carrier
	prop.Inject(ctx, carrier)

	// Log what was injected
	envVars := carrier.ToEnv()
	logger.Debug(ctx, "Trace context injected",
		"envVars", envVars,
		"carrier", carrier.env,
	)

	// Convert to environment variables
	return envVars
}

// ExtractTraceContext extracts trace context from environment variables
func ExtractTraceContext(ctx context.Context) context.Context {
	// Get the global propagator
	prop := otel.GetTextMapPropagator()

	// Create a carrier and populate it from environment variables
	carrier := NewTraceContextCarrier()

	// Look for W3C trace context environment variables
	// W3C spec uses lowercase keys internally, but check both cases for environment variables
	if traceparent := os.Getenv("TRACEPARENT"); traceparent != "" {
		carrier.Set("traceparent", traceparent)
	} else if traceparent := os.Getenv("traceparent"); traceparent != "" {
		carrier.Set("traceparent", traceparent)
	}

	if tracestate := os.Getenv("TRACESTATE"); tracestate != "" {
		carrier.Set("tracestate", tracestate)
	} else if tracestate := os.Getenv("tracestate"); tracestate != "" {
		carrier.Set("tracestate", tracestate)
	}

	logger.Debug(ctx, "ExtractTraceContext called",
		"TRACEPARENT", os.Getenv("TRACEPARENT"),
		"traceparent", os.Getenv("traceparent"),
		"TRACESTATE", os.Getenv("TRACESTATE"),
		"tracestate", os.Getenv("tracestate"),
		"carrier", carrier.env,
	)

	// Extract the trace context from the carrier
	newCtx := prop.Extract(ctx, carrier)

	// Check if extraction worked
	span := trace.SpanFromContext(newCtx)
	spanCtx := span.SpanContext()
	logger.Debug(ctx, "Trace context extracted",
		"hasActiveSpan", spanCtx.IsValid(),
		"traceID", spanCtx.TraceID().String(),
		"spanID", spanCtx.SpanID().String(),
		"traceFlags", spanCtx.TraceFlags(),
	)

	return newCtx
}
