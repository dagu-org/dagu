package admin

import (
	"jobctl/internal/admin/handlers"
	"net/http"
)

type route struct {
	method  string
	pattern string
	handler http.HandlerFunc
}

func defaultRoutes(cfg *Config) []*route {
	return []*route{
		{http.MethodGet, `^/?$`, handlers.HandleGetList(
			&handlers.JobListHandlerConfig{
				JobsDir: cfg.Jobs,
			},
		)},
		{http.MethodGet, `^/([^/]+)$`, handlers.HandleGetJob(
			&handlers.JobHandlerConfig{
				JobsDir:            cfg.Jobs,
				LogEncodingCharset: cfg.LogEncodingCharset,
			},
		)},
		{http.MethodPost, `^/([^/]+)$`, handlers.HandlePostJobAction(
			&handlers.PostJobHandlerConfig{
				JobsDir: cfg.Jobs,
				Bin:     cfg.Command,
				WkDir:   cfg.WorkDir,
			},
		)},
	}
}
