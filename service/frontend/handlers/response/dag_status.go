package response

import (
	domain "github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/samber/lo"
)

func ToDagStatus(s *domain.Status) *models.DagStatus {
	return &models.DagStatus{
		Log:        lo.ToPtr(s.Log),
		Name:       lo.ToPtr(s.Name),
		Params:     lo.ToPtr(s.Params),
		Pid:        lo.ToPtr(int64(s.Pid)),
		RequestID:  lo.ToPtr(s.RequestId),
		StartedAt:  lo.ToPtr(s.StartedAt),
		FinishedAt: lo.ToPtr(s.FinishedAt),
		Status:     lo.ToPtr(int64(s.Status)),
		StatusText: lo.ToPtr(s.StatusText),
	}
}
