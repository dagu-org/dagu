package handlers

import (
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"

	"github.com/yohamta/dagman/internal/controller"
	"github.com/yohamta/dagman/internal/utils"
)

type dagListResponse struct {
	Title    string
	Charset  string
	DAGs     []*controller.DAG
	Groups   []*group
	Group    string
	Errors   []string
	HasError bool
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
		}
		if r.Header.Get("Accept") == "application/json" {
			renderJson(w, data)
		} else {
			renderFunc(w, data)
		}
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
	fis, err := ioutil.ReadDir(dir)
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
