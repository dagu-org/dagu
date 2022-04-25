package handlers

import (
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"

	"github.com/yohamta/jobctl/internal/controller"
)

type jobListResponse struct {
	Title    string
	Charset  string
	Jobs     []*controller.Job
	Groups   []*group
	Group    string
	Errors   []string
	HasError bool
}

type jobListParameter struct {
	Group string
}

type group struct {
	Name string
	Dir  string
}

type JobListHandlerConfig struct {
	JobsDir string
}

func HandleGetList(hc *JobListHandlerConfig) http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "index")
	return func(w http.ResponseWriter, r *http.Request) {
		params := getGetListParameter(r)
		dir := filepath.Join(hc.JobsDir, params.Group)
		jobs, errs, err := controller.GetJobList(dir)
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
		for _, j := range jobs {
			if j.Error != nil {
				hasErr = true
				break
			}
		}
		if len(errs) > 0 {
			hasErr = true
		}

		data := &jobListResponse{
			Title:    "JobList",
			Jobs:     jobs,
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

func getGetListParameter(r *http.Request) *jobListParameter {
	p := &jobListParameter{
		Group: "",
	}
	if group, ok := r.URL.Query()["group"]; ok {
		p.Group = group[0]
	}
	return p
}

func listGroups(dir string) ([]*group, error) {
	ret := []*group{}

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
