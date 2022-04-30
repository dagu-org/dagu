package admin

import (
	"net/http"

	"github.com/yohamta/dagman/internal/admin/handlers"
)

type route struct {
	method  string
	pattern string
	handler http.HandlerFunc
}

func defaultRoutes(cfg *Config) []*route {
	return []*route{
		{http.MethodGet, `^/?$`, handlers.HandleGetList(
			&handlers.DAGListHandlerConfig{
				DAGsDir: cfg.DAGs,
			},
		)},
		{http.MethodGet, `^/dags/?$`, handlers.HandleGetList(
			&handlers.DAGListHandlerConfig{
				DAGsDir: cfg.DAGs,
			},
		)},
		{http.MethodGet, `^/dags/([^/]+)$`, handlers.HandleGetDAG(
			&handlers.DAGHandlerConfig{
				DAGsDir:            cfg.DAGs,
				LogEncodingCharset: cfg.LogEncodingCharset,
			},
		)},
		{http.MethodPost, `^/dags/([^/]+)$`, handlers.HandlePostDAGAction(
			&handlers.PostDAGHandlerConfig{
				DAGsDir: cfg.DAGs,
				Bin:     cfg.Command,
				WkDir:   cfg.WorkDir,
			},
		)},
	}
}
