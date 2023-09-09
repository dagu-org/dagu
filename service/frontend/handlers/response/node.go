package response

import (
	domain "github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/samber/lo"
)

func ToNode(node *domain.Node) *models.StatusNode {
	return &models.StatusNode{
		DoneCount:  lo.ToPtr(int64(node.DoneCount)),
		Error:      lo.ToPtr(node.Error),
		FinishedAt: lo.ToPtr(node.FinishedAt),
		Log:        lo.ToPtr(node.Log),
		RetryCount: lo.ToPtr(int64(node.RetryCount)),
		StartedAt:  lo.ToPtr(node.StartedAt),
		Status:     lo.ToPtr(int64(node.Status)),
		StatusText: lo.ToPtr(node.StatusText),
		Step:       ToStepObject(node.Step),
	}
}
