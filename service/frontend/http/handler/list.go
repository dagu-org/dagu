package handlers

import (
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"net/http"
	"path"
)

func handleIndex() http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "index")
	return func(w http.ResponseWriter, r *http.Request) {
		renderFunc(w, nil)
	}
}

func handlePostList() http.HandlerFunc {
	cfg := config.Get()

	return func(w http.ResponseWriter, r *http.Request) {
		action := r.FormValue("action")
		value := r.FormValue("value")

		switch action {
		case "new":
			filename := nameWithExt(path.Join(cfg.DAGs, value))
			err := controller.CreateDAG(filename)
			if err != nil {
				encodeError(w, err)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
			return
		}
		encodeError(w, errInvalidArgs)
	}
}
