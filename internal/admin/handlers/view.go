package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"

	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/filters"
	"github.com/yohamta/dagu/internal/views"
)

type viewResponse struct {
	Title    string
	Charset  string
	DAGs     []*controller.DAG
	Errors   []string
	HasError bool
}

type ViewHandlerConfig struct {
	DAGsDir string
}

func HandleGetView(hc *ViewHandlerConfig) http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "index")

	return func(w http.ResponseWriter, r *http.Request) {
		p, err := getViewParameter(r)
		if err != nil {
			encodeError(w, err)
			return
		}

		view, err := views.GetView(p)
		if err != nil {
			encodeError(w, err)
			return
		}

		dir := filepath.Join(hc.DAGsDir, "")
		dags, errs, err := controller.GetDAGs(dir)
		if err != nil {
			encodeError(w, err)
			return
		}

		filteredDags := []*controller.DAG{}
		filter := &filters.ContainTags{
			Tags: view.ContainTags,
		}
		for _, d := range dags {
			if filter.Matches(d.Config) {
				filteredDags = append(filteredDags, d)
			}
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

		data := &viewResponse{
			Title:    "View",
			DAGs:     filteredDags,
			Errors:   errs,
			HasError: hasErr,
		}

		if isJsonRequest(r) {
			renderJson(w, data)
		} else {
			renderFunc(w, data)
		}
	}
}

func getViewParameter(r *http.Request) (string, error) {
	re := regexp.MustCompile(`/views/([^/\?]+)/?$`)
	m := re.FindStringSubmatch(r.URL.Path)
	if len(m) < 2 {
		return "", fmt.Errorf("invalid URL")
	}
	return m[1], nil
}
