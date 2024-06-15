package response

import (
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/service/frontend/gen/models"
	"github.com/go-openapi/swag"
)

func NewDagStatus(status *model.Status) *models.DagStatus {
	return &models.DagStatus{
		Log:        swag.String(status.Log),
		Name:       swag.String(status.Name),
		Params:     swag.String(status.Params),
		Pid:        swag.Int64(int64(status.Pid)),
		RequestID:  swag.String(status.RequestId),
		StartedAt:  swag.String(status.StartedAt),
		FinishedAt: swag.String(status.FinishedAt),
		Status:     swag.Int64(int64(status.Status)),
		StatusText: swag.String(status.StatusText),
	}
}
