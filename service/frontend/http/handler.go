package http

import "github.com/dagu-dev/dagu/service/frontend/restapi/operations"

type Handler interface {
	Configure(api *operations.DaguAPI)
}
