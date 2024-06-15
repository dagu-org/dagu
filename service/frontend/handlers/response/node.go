package response

import (
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/service/frontend/models"
	"github.com/go-openapi/swag"
)

func ToNode(node *model.Node) *models.StatusNode {
	return &models.StatusNode{
		DoneCount:  swag.Int64(int64(node.DoneCount)),
		Error:      swag.String(node.Error),
		FinishedAt: swag.String(node.FinishedAt),
		Log:        swag.String(node.Log),
		RetryCount: swag.Int64(int64(node.RetryCount)),
		StartedAt:  swag.String(node.StartedAt),
		Status:     swag.Int64(int64(node.Status)),
		StatusText: swag.String(node.StatusText),
		Step:       ToStepObject(node.Step),
	}
}
