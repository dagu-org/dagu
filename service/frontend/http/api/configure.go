package api

import (
	"github.com/yohamta/dagu/service/frontend/restapi/operations"
)

func Configure(api *operations.DaguAPI) {
	registerWorkflows(api)
}
