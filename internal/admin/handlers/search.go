package handlers

import (
	"net/http"

	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
)

type searchResponse struct {
	Results []*controller.GrepResult
	Errors  []string
}

func handleGetSearch() http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "search")
	cfg := config.Get()

	return func(w http.ResponseWriter, r *http.Request) {
		query, ok := r.URL.Query()["q"]
		if !ok || len(query) == 0 || query[0] == "" {
			encodeError(w, errInvalidArgs)
			return
		}

		ret, errs, err := controller.GrepDAG(cfg.DAGs, query[0])
		if err != nil {
			encodeError(w, err)
			return
		}

		resp := &searchResponse{
			Results: ret,
			Errors:  errs,
		}

		if isJsonRequest(r) {
			renderJson(w, resp)
		} else {
			renderFunc(w, resp)
		}
	}
}
