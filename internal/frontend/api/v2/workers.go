package api

import (
	"context"

	"github.com/dagu-org/dagu/api/v2"
)

// WorkerPoll implements api.StrictServerInterface.
func (a *API) WorkerPoll(ctx context.Context, request api.WorkerPollRequestObject) (api.WorkerPollResponseObject, error) {
	panic("unimplemented")
}
