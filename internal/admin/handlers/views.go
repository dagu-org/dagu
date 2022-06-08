package handlers

import (
	"encoding/json"
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
			Title: "Views",
			Views: views.GetViews(),
		}
		if r.Header.Get("Accept") == "application/json" {
			renderJson(w, data)
		} else {
			renderFunc(w, data)
		}
	}
}

func HandlePutView() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		var v models.View
		err := json.NewDecoder(r.Body).Decode(&v)
		if err != nil {
			encodeError(w, err)
			return
		}

		err = views.SaveView(&v)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}
