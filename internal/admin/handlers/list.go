package handlers

import (
	"fmt"
	"github.com/yohamta/dagu/internal/controller"
	"net/http"
	"path"
	"path/filepath"
)

type dagListResponse struct {
	Title    string
	Charset  string
	DAGs     []*controller.DAGStatus
	Errors   []string
	HasError bool
}

type DAGListHandlerConfig struct {
	DAGsDir string
}

func HandleGetList(hc *DAGListHandlerConfig, tc *TemplateConfig) http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "index", tc)
	return func(w http.ResponseWriter, r *http.Request) {
		dir := filepath.Join(hc.DAGsDir)
		dr := controller.NewDAGStatusReader()
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

func HandlePostList(hc *DAGListHandlerConfig) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		action := r.FormValue("action")
		value := r.FormValue("value")

		switch action {
		case "new":
			filename := nameWithExt(path.Join(hc.DAGsDir, value))
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

func nameWithExt(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return fmt.Sprintf("%s.yaml", name)
	}
	return name
}
