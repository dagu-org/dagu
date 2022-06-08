package handlers

import (
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/utils"
	"github.com/yohamta/dagu/internal/views"
)

type dagListResponse struct {
	Title    string
	Charset  string
	DAGs     []*controller.DAG
	Groups   []*group
	Group    string
	Errors   []string
	HasError bool
	Views    []*models.View
}

type dagListParameter struct {
	Group string
}

type group struct {
	Name string
	Dir  string
}

type DAGListHandlerConfig struct {
	DAGsDir string
}

func HandleGetList(hc *DAGListHandlerConfig) http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "index")
	return func(w http.ResponseWriter, r *http.Request) {
		params := getGetListParameter(r)
		dir := filepath.Join(hc.DAGsDir, params.Group)
		dags, errs, err := controller.GetDAGs(dir)
		if err != nil {
			encodeError(w, err)
			return
		}

		groups := []*group{}
		if params.Group == "" {
			groups, err = listGroups(dir)
			if err != nil {
				encodeError(w, err)
				return
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

		data := &dagListResponse{
			Title:    "DAGList",
			DAGs:     dags,
			Groups:   groups,
			Group:    params.Group,
			Errors:   errs,
			HasError: hasErr,
			Views:    views.GetViews(),
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
		group := r.FormValue("group")

		switch action {
		case "new":
			filename := path.Join(hc.DAGsDir, group, value)
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

func getGetListParameter(r *http.Request) *dagListParameter {
	p := &dagListParameter{
		Group: "",
	}
	if group, ok := r.URL.Query()["group"]; ok {
		p.Group = group[0]
	}
	return p
}

func listGroups(dir string) ([]*group, error) {
	ret := []*group{}
	if !utils.FileExists(dir) {
		return ret, nil
	}
	fis, err := os.ReadDir(dir)
	if err != nil || fis == nil {
		log.Printf("%v", err)
	}
	for _, fi := range fis {
		if !fi.IsDir() {
			continue
		}
		ret = append(ret, &group{
			fi.Name(), filepath.Join(dir, fi.Name()),
		})
	}
	return ret, nil
}
