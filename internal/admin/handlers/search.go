package handlers

import (
	"net/http"

	"github.com/yohamta/dagu/internal/controller"
)

type searchResponse struct {
	Result []*controller.GrepResult
	Errors []string
}

func HandleGetSearch(DAGsDir string, tc *TemplateConfig) http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "search", tc)

	return func(w http.ResponseWriter, r *http.Request) {
		query, ok := r.URL.Query()["q"]
		if !ok || len(query) == 0 {
			encodeError(w, errInvalidArgs)
			return
		}

		ret, errs, err := controller.GrepDAGs(DAGsDir, query[0])
		if err != nil {
			encodeError(w, err)
			return
		}

		resp := &searchResponse{
			Result: ret,
			Errors: errs,
		}

		if isJsonRequest(r) {
			renderJson(w, resp)
		} else {
			renderFunc(w, resp)
		}
	}
}
