package api

import (
	"github.com/go-openapi/runtime/middleware"
	"github.com/yohamta/dagu/service/frontend/http/api/workflow"
	"github.com/yohamta/dagu/service/frontend/restapi/operations"
)

func Configure(api *operations.DaguAPI) {
	api.ListWorkflowsHandler = operations.ListWorkflowsHandlerFunc(
		func(params operations.ListWorkflowsParams) middleware.Responder {
			resp, err := workflow.GetList(params)
			if err != nil {
				return operations.NewListWorkflowsDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewListWorkflowsOK().WithPayload(resp)
		})

	api.GetWorkflowHandler = operations.GetWorkflowHandlerFunc(
		func(params operations.GetWorkflowParams) middleware.Responder {
			resp, err := workflow.GetDetail(params)
			if err != nil {
				return operations.NewGetWorkflowDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewGetWorkflowOK().WithPayload(resp)
		})

	api.PostWorkflowActionHandler = operations.PostWorkflowActionHandlerFunc(
		func(params operations.PostWorkflowActionParams) middleware.Responder {
			resp, err := workflow.PostAction(params)
			if err != nil {
				return operations.NewPostWorkflowActionDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewPostWorkflowActionOK().WithPayload(resp)
		})
}
