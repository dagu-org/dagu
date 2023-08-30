package response

import (
	"github.com/samber/lo"
	domain "github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/service/frontend/models"
)

func ToWorkflowStatus(s *domain.Status) *models.WorkflowStatus {
	return &models.WorkflowStatus{
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
