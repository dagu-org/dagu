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

	api.GetWorkflowDetailHandler = operations.GetWorkflowDetailHandlerFunc(
		func(params operations.GetWorkflowDetailParams) middleware.Responder {
			resp, err := workflow.GetDetail(params)
			if err != nil {
				return operations.NewGetWorkflowDetailDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewGetWorkflowDetailOK().WithPayload(resp)
		})

	api.PostWorkflowActionHandler = operations.PostWorkflowActionHandlerFunc(
		func(params operations.PostWorkflowActionParams) middleware.Responder {
			resp, err := workflow.PostAction(params)
			if err != nil {
				return operations.NewPostWorkflowActionDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewPostWorkflowActionOK().WithPayload(resp)
		})

	api.CreateWorkflowHandler = operations.CreateWorkflowHandlerFunc(
		func(params operations.CreateWorkflowParams) middleware.Responder {
			resp, err := workflow.Create(params)
			if err != nil {
				return operations.NewCreateWorkflowDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewCreateWorkflowOK().WithPayload(resp)
		})

	api.DeleteWorkflowHandler = operations.DeleteWorkflowHandlerFunc(
		func(params operations.DeleteWorkflowParams) middleware.Responder {
			err := workflow.Delete(params)
			if err != nil {
				return operations.NewDeleteWorkflowDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewDeleteWorkflowOK()
		})
}
