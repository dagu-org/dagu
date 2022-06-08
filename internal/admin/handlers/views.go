package handlers

import (
	"net/http"

	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/views"
)

type viewListResponse struct {
	Title   string
	Charset string
	Views   []*models.View
}

func HandleGetViewList() http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "index")
	return func(w http.ResponseWriter, r *http.Request) {
		data := &viewListResponse{
			Title: "DAGList",
			Views: views.GetViews(),
		}
		if r.Header.Get("Accept") == "application/json" {
			renderJson(w, data)
		} else {
			renderFunc(w, data)
		}
	}
}
