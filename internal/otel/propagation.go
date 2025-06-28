package otel

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
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
	return c.env[key]
}

// Set stores the key-value pair
func (c *TraceContextCarrier) Set(key string, value string) {
	c.env[key] = value
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
	// Get the global propagator
	prop := otel.GetTextMapPropagator()

	// Create a carrier for the trace context
	carrier := NewTraceContextCarrier()

	// Inject the trace context into the carrier
	prop.Inject(ctx, carrier)

	// Convert to environment variables
	return carrier.ToEnv()
}

// ExtractTraceContext extracts trace context from environment variables
func ExtractTraceContext(ctx context.Context) context.Context {
	// Get the global propagator
	prop := otel.GetTextMapPropagator()

	// Create a carrier and populate it from environment variables
	carrier := NewTraceContextCarrier()

	// Look for W3C trace context environment variables
	if traceparent := os.Getenv("TRACEPARENT"); traceparent != "" {
		carrier.Set("traceparent", traceparent)
	}
	if tracestate := os.Getenv("TRACESTATE"); tracestate != "" {
		carrier.Set("tracestate", tracestate)
	}

	// Extract the trace context from the carrier
	return prop.Extract(ctx, carrier)
}
