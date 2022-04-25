package admin

import (
	"net/http"

	"github.com/yohamta/jobctl/internal/admin/handlers"
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
		{http.MethodGet, `^/jobs/?$`, handlers.HandleGetList(
			&handlers.JobListHandlerConfig{
				JobsDir: cfg.Jobs,
			},
		)},
		{http.MethodGet, `^/jobs/([^/]+)$`, handlers.HandleGetJob(
			&handlers.JobHandlerConfig{
				JobsDir:            cfg.Jobs,
				LogEncodingCharset: cfg.LogEncodingCharset,
			},
		)},
		{http.MethodPost, `^/jobs/([^/]+)$`, handlers.HandlePostJobAction(
			&handlers.PostJobHandlerConfig{
				JobsDir: cfg.Jobs,
				Bin:     cfg.Command,
				WkDir:   cfg.WorkDir,
			},
		)},
	}
}
