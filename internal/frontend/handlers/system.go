package handlers

import (
	"net/http"
	"time"

	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/frontend/gen/models"
	"github.com/dagu-org/dagu/internal/frontend/gen/restapi/operations"
	"github.com/dagu-org/dagu/internal/frontend/gen/restapi/operations/system"
	"github.com/dagu-org/dagu/internal/frontend/metrics"
	"github.com/dagu-org/dagu/internal/frontend/server"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/swag"
)

var _ server.Handler = (*System)(nil)

// System is a handler for system related operations.
type System struct{}

// Configure implements server.Handler.
func (s *System) Configure(api *operations.DaguAPI) {
	api.SystemGetHealthHandler = system.GetHealthHandlerFunc(func(ghp system.GetHealthParams) middleware.Responder {
		resp, err := s.GetHealth(ghp)
		if err != nil {
			return system.NewGetHealthDefault(http.StatusBadGateway)
		}
		return system.NewGetHealthOK().WithPayload(resp)
	})
}

func NewSystem() server.Handler {
	return &System{}
}

func (s *System) GetHealth(_ system.GetHealthParams) (*models.HealthResponse, error) {
	return &models.HealthResponse{
		Status:    swag.String(models.HealthResponseStatusHealthy),
		Version:   &build.Version,
		Uptime:    swag.Int64(metrics.GetUptime()),
		Timestamp: swag.String(stringutil.FormatTime(time.Now())),
	}, nil
}
