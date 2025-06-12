package api

import (
	"context"
	"net/http"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/metrics"
	"github.com/dagu-org/dagu/internal/scheduler"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// initMetrics initializes the metrics collector and registry on first call
func (a *API) initMetrics() {
	if a.metricsRegistry != nil {
		return // Already initialized
	}

	collector := metrics.NewCollector(
		build.Version,
		"",  // Build date not available in current build package
		scheduler.IsSchedulerRunning, // Use global scheduler status
		a.dagStore,
		a.dagRunStore,
		nil, // Queue store not available in API struct yet
	)

	a.metricsRegistry = metrics.NewRegistry(collector)
}

func (a *API) GetMetrics(_ context.Context, _ api.GetMetricsRequestObject) (api.GetMetricsResponseObject, error) {
	// Initialize metrics on first call
	a.initMetrics()

	// Use promhttp handler to write metrics
	handler := promhttp.HandlerFor(a.metricsRegistry, promhttp.HandlerOpts{})

	// Create a custom response writer that implements the API response interface
	return &MetricsTextResponse{
		handler: handler,
	}, nil
}

// MetricsTextResponse implements the response interface for metrics
type MetricsTextResponse struct {
	handler http.Handler
}

func (r *MetricsTextResponse) VisitGetMetricsResponse(w http.ResponseWriter) error {
	// Set proper content type for Prometheus metrics
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	// Use the promhttp handler to write the metrics
	r.handler.ServeHTTP(w, &http.Request{})

	return nil
}

