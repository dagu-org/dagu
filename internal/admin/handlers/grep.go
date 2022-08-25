package handlers

import (
	"net/http"

	"github.com/yohamta/dagu/internal/controller"
)

type grepResponse struct {
	Result []*controller.GrepResult
	Errors []string
}

func HandleGetGrep(DAGsDir string, tc *TemplateConfig) http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "grep", tc)

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

		resp := &grepResponse{
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
