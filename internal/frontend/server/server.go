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
	"github.com/jessevdk/go-flags"

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
	tls         *config.TLSConfig
	server      *restapi.Server
	handlers    []Handler
	assets      fs.FS
	headless    bool
}

type NewServerArgs struct {
	Host      string
	Port      int
	BasicAuth *BasicAuth
	AuthToken *AuthToken
	TLS       *config.TLSConfig
	Handlers  []Handler
	AssetsFS  fs.FS

	Headless              bool
	NavbarColor           string
	NavbarTitle           string
	BasePath              string
	APIBaseURL            string
	TimeZone              string
	MaxDashboardPageLimit int
	RemoteNodes           []string
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
		handlers:  params.Handlers,
		assets:    params.AssetsFS,
		headless:  params.Headless, // Assign headless mode flag
		funcsConfig: funcsConfig{
			NavbarColor:           params.NavbarColor,
			NavbarTitle:           params.NavbarTitle,
			BasePath:              params.BasePath,
			APIBaseURL:            params.APIBaseURL,
			TZ:                    params.TimeZone,
			MaxDashboardPageLimit: params.MaxDashboardPageLimit,
			RemoteNodes:           params.RemoteNodes,
		},
	}
}

func (svr *Server) Shutdown(ctx context.Context) {
	if svr.server == nil {
		return
	}
	err := svr.server.Shutdown()
	if err != nil {
		logger.Warn(ctx, "Server shutdown", "error", err)
	}
}

func (svr *Server) Serve(ctx context.Context) (err error) {
	loggerInstance := logger.FromContext(ctx)

	// Setup middleware & routes
	middlewareOptions := &pkgmiddleware.Options{
		Handler:  svr.defaultRoutes(ctx, chi.NewRouter()), // API remains active
		BasePath: svr.funcsConfig.BasePath,
		Logger:   loggerInstance,
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

	// Load API spec (Always required)
	swaggerSpec, err := loads.Analyzed(restapi.SwaggerJSON, "")
	if err != nil {
		logger.Error(ctx, "Failed to load API spec", "err", err)
		return err
	}
	api := operations.NewDaguAPI(swaggerSpec)
	api.Logger = loggerInstance.Infof
	for _, h := range svr.handlers {
		h.Configure(api) // Always configure API handlers
	}

	// Start API server
	svr.server = restapi.NewServer(api)
	defer svr.Shutdown(ctx)

	svr.server.Host = svr.host
	svr.server.Port = svr.port
	svr.server.ConfigureAPI()

	// Listen for system signals (CTRL+C, termination)
	serverCtx, serverStopCtx := context.WithCancel(ctx)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig
		_ = svr.server.Shutdown()
		serverStopCtx()
	}()

	// Run with or without TLS
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
		logger.Error(ctx, "Server error", "err", err)
	}

	<-serverCtx.Done()
	logger.Info(ctx, "Server stopped")
	return nil
}
