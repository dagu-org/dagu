package api

import (
	"github.com/dagu-dev/dagu/service/frontend/http/api/workflow"
	"github.com/dagu-dev/dagu/service/frontend/restapi/operations"
	"github.com/go-openapi/runtime/middleware"
)

func Configure(api *operations.DaguAPI) {
	api.ListDagsHandler = operations.ListDagsHandlerFunc(
		func(params operations.ListDagsParams) middleware.Responder {
			resp, err := workflow.GetList(params)
			if err != nil {
				return operations.NewListDagsDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewListDagsOK().WithPayload(resp)
		})

	api.GetDagDetailHandler = operations.GetDagDetailHandlerFunc(
		func(params operations.GetDagDetailParams) middleware.Responder {
			resp, err := workflow.GetDetail(params)
			if err != nil {
				return operations.NewGetDagDetailDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewGetDagDetailOK().WithPayload(resp)
		})

	api.PostDagActionHandler = operations.PostDagActionHandlerFunc(
		func(params operations.PostDagActionParams) middleware.Responder {
			resp, err := workflow.PostAction(params)
			if err != nil {
				return operations.NewPostDagActionDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewPostDagActionOK().WithPayload(resp)
		})

	api.CreateDagHandler = operations.CreateDagHandlerFunc(
		func(params operations.CreateDagParams) middleware.Responder {
			resp, err := workflow.Create(params)
			if err != nil {
				return operations.NewCreateDagDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewCreateDagOK().WithPayload(resp)
		})

	api.DeleteDagHandler = operations.DeleteDagHandlerFunc(
		func(params operations.DeleteDagParams) middleware.Responder {
			err := workflow.Delete(params)
			if err != nil {
				return operations.NewDeleteDagDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewDeleteDagOK()
		})

	api.SearchDagsHandler = operations.SearchDagsHandlerFunc(
		func(params operations.SearchDagsParams) middleware.Responder {
			resp, err := workflow.Search(params)
			if err != nil {
				return operations.NewSearchDagsDefault(err.Code).WithPayload(err.APIError)
			}
			return operations.NewSearchDagsOK().WithPayload(resp)
		})
}
