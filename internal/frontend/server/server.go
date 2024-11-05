// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package server

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/frontend/gen/restapi"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/go-openapi/loads"
	flags "github.com/jessevdk/go-flags"

	"github.com/dagu-org/dagu/internal/frontend/gen/restapi/operations"
	pkgmiddleware "github.com/dagu-org/dagu/internal/frontend/middleware"

	"github.com/go-chi/chi/v5"
)

type Server struct {
	funcsConfig funcsConfig
	host        string
	port        int
	basicAuth   *BasicAuth
	authToken   *AuthToken
	tls         *config.TLS
	logger      logger.Logger
	server      *restapi.Server
	handlers    []Handler
	assets      fs.FS
}

type NewServerArgs struct {
	Host      string
	Port      int
	BasicAuth *BasicAuth
	AuthToken *AuthToken
	TLS       *config.TLS
	Logger    logger.Logger
	Handlers  []Handler
	AssetsFS  fs.FS

	// Configuration for the frontend
	NavbarColor string
	NavbarTitle string
	APIBaseURL  string
	TimeZone  string
}

type BasicAuth struct {
	Username string
	Password string
}

type AuthToken struct {
	Token string
}

type Handler interface {
	Configure(api *operations.DaguAPI)
}

func New(params NewServerArgs) *Server {
	return &Server{
		host:      params.Host,
		port:      params.Port,
		basicAuth: params.BasicAuth,
		authToken: params.AuthToken,
		tls:       params.TLS,
		logger:    params.Logger,
		handlers:  params.Handlers,
		assets:    params.AssetsFS,
		funcsConfig: funcsConfig{
			NavbarColor: params.NavbarColor,
			NavbarTitle: params.NavbarTitle,
			APIBaseURL:  params.APIBaseURL,
			TimeZone:  params.TimeZone,
		},
	}
}

func (svr *Server) Shutdown() {
	if svr.server == nil {
		return
	}
	err := svr.server.Shutdown()
	if err != nil {
		svr.logger.Warn("Server shutdown", "error", err)
	}
}

func (svr *Server) Serve(ctx context.Context) (err error) {
	middlewareOptions := &pkgmiddleware.Options{
		Handler: svr.defaultRoutes(chi.NewRouter()),
		Logger:  svr.logger,
	}
	if svr.authToken != nil {
		middlewareOptions.AuthToken = &pkgmiddleware.AuthToken{
			Token: svr.authToken.Token,
		}
	}
	if svr.basicAuth != nil {
		middlewareOptions.AuthBasic = &pkgmiddleware.AuthBasic{
			Username: svr.basicAuth.Username,
			Password: svr.basicAuth.Password,
		}
	}
	pkgmiddleware.Setup(middlewareOptions)

	swaggerSpec, err := loads.Analyzed(restapi.SwaggerJSON, "")
	if err != nil {
		svr.logger.Error("Failed to load API spec", "error", err)
		return err
	}
	api := operations.NewDaguAPI(swaggerSpec)
	api.Logger = svr.logger.Infof
	for _, h := range svr.handlers {
		h.Configure(api)
	}

	svr.server = restapi.NewServer(api)
	defer svr.Shutdown()

	svr.server.Host = svr.host
	svr.server.Port = svr.port
	svr.server.ConfigureAPI()

	// Server run context
	serverCtx, serverStopCtx := context.WithCancel(ctx)

	// Listen for syscall signals for process to interrupt/quit
	sig := make(chan os.Signal, 1)
	signal.Notify(
		sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT,
	)
	go func() {
		<-sig

		// Trigger graceful shutdown
		err := svr.server.Shutdown()
		if err != nil {
			svr.logger.Error("Server shutdown", "error", err)
		}
		serverStopCtx()
	}()

	if svr.tls != nil {
		svr.server.TLSCertificate = flags.Filename(svr.tls.CertFile)
		svr.server.TLSCertificateKey = flags.Filename(svr.tls.KeyFile)
		svr.server.EnabledListeners = []string{"https"}
		svr.server.TLSHost = svr.host
		svr.server.TLSPort = svr.port
	}

	// Run the server
	err = svr.server.Serve()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		svr.logger.Error("Server error", "error", err)
	}

	// Wait for server context to be stopped
	<-serverCtx.Done()

	svr.logger.Info("Server stopped")

	return nil
}
