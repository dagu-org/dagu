package handlers

import (
	"net/http"
	"path"
	"path/filepath"

	"github.com/yohamta/dagu/internal/controller"
)

type dagListResponse struct {
	Title    string
	Charset  string
	DAGs     []*controller.DAG
	Errors   []string
	HasError bool
}

type DAGListHandlerConfig struct {
	DAGsDir string
}

func HandleGetList(hc *DAGListHandlerConfig) http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "index")
	return func(w http.ResponseWriter, r *http.Request) {
		dir := filepath.Join(hc.DAGsDir)
		dags, errs, err := controller.GetDAGs(dir)
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

func HandlePostList(hc *DAGListHandlerConfig) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		action := r.FormValue("action")
		value := r.FormValue("value")

		switch action {
		case "new":
			filename := path.Join(hc.DAGsDir, value)
			err := controller.NewConfig(filename)
			if err != nil {
				encodeError(w, err)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}
		encodeError(w, errInvalidArgs)
	}
}
