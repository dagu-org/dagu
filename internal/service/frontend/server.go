package frontend

import (
	"github.com/dagu-dev/dagu/internal/service/frontend/dag"
	"github.com/dagu-dev/dagu/internal/service/frontend/server"
)

func NewServer(params Params) *server.Server {

	var hs []server.Handler

	hs = append(hs, dag.NewHandler(
		&dag.NewHandlerArgs{
			Engine:             params.Engine,
			LogEncodingCharset: params.Config.LogEncodingCharset,
		},
	))

	serverParams := server.Params{
		Host:     params.Config.Host,
		Port:     params.Config.Port,
		TLS:      params.Config.TLS,
		Logger:   params.Logger,
		Handlers: hs,
		AssetsFS: assetsFS,
	}

	if params.Config.IsAuthToken {
		serverParams.AuthToken = &server.AuthToken{
			Token: params.Config.AuthToken,
		}
	}

	if params.Config.IsBasicAuth {
		serverParams.BasicAuth = &server.BasicAuth{
			Username: params.Config.BasicAuthUsername,
			Password: params.Config.BasicAuthPassword,
		}
	}

	return server.NewServer(serverParams, params.Config)
}
