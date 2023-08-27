package handlers

import (
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"net/http"
	"path"
	"path/filepath"

	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
)

type dagListResponse struct {
	Title    string
	Charset  string
	DAGs     []*controller.DAGStatus
	Errors   []string
	HasError bool
}

func handleGetList() http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "index")
	cfg := config.Get()
	return func(w http.ResponseWriter, r *http.Request) {
		dir := filepath.Join(cfg.DAGs)
		dr := controller.NewDAGStatusReader(jsondb.New())
		dags, errs, err := dr.ReadAllStatus(dir)
		if err != nil {
			encodeError(w, err)
			return
		}

		hasErr := false
		for _, j := range dags {
			if j.Error != nil {
				hasErr = true
				break
			}
		}
		if len(errs) > 0 {
			hasErr = true
		}

		data := &dagListResponse{
			Title:    "DAGList",
			DAGs:     dags,
			Errors:   errs,
			HasError: hasErr,
		}
		if r.Header.Get("Accept") == "application/json" {
			renderJson(w, data)
		} else {
			renderFunc(w, data)
		}
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
