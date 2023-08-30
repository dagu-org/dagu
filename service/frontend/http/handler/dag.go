package handlers

import (
	"fmt"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"net/http"
	"path/filepath"
)

func handleGetDAG() http.HandlerFunc {
	renderFunc := useTemplate("index.gohtml", "dag")
	return func(w http.ResponseWriter, r *http.Request) {
		renderFunc(w, nil)
	}
}

func handleDeleteDAG() http.HandlerFunc {
	cfg := config.Get()

	return func(w http.ResponseWriter, r *http.Request) {
		dn := dagNameFromCtx(r.Context())

		file := filepath.Join(cfg.DAGs, fmt.Sprintf("%s.yaml", dn))
		dr := controller.NewDAGStatusReader(jsondb.New())
		d, err := dr.ReadStatus(file, false)
		if err != nil {
			encodeError(w, err)
		}

		c := controller.New(d.DAG, jsondb.New())

		err = c.DeleteDAG()

		if err != nil {
			encodeError(w, err)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}
}
