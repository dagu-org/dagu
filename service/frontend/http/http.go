package http

import (
	"context"
	"errors"
	"fmt"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/logger"
	"github.com/yohamta/dagu/internal/logger/tag"
	"github.com/yohamta/dagu/service/frontend/http/handler"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/yohamta/dagu/internal/utils"
)

type ServerParams struct {
	Host      string
	Port      int
	BasicAuth *BasicAuth
	TLS       *config.TLS
	Logger    logger.Logger
}

type BasicAuth struct {
	Username string
	Password string
}

type Server struct {
	host            string
	port            int
	basicAuth       *BasicAuth
	tls             *config.TLS
	addr            string
	server          *http.Server
	idleConnsClosed chan struct{}
	logger          logger.Logger
}

func NewServer(params ServerParams) *Server {
	return &Server{
		addr:            net.JoinHostPort(params.Host, strconv.Itoa(params.Port)),
		idleConnsClosed: nil,
		host:            params.Host,
		port:            params.Port,
		basicAuth:       params.BasicAuth,
		tls:             params.TLS,
		logger:          params.Logger,
	}
}

func (svr *Server) Shutdown(_ context.Context) error {
	err := svr.server.Shutdown(context.Background())
	if err != nil {
		svr.logger.Warn("Server shutdown", tag.Error(err))
	}
	if svr.idleConnsClosed != nil {
		close(svr.idleConnsClosed)
		svr.idleConnsClosed = nil
	}
	return nil
}

func (svr *Server) Signal(_ os.Signal) {
	_ = svr.Shutdown(context.Background())
}

func (svr *Server) Start() (err error) {
	svr.setupServer()
	svr.setupHandler()

	svr.idleConnsClosed = make(chan struct{})
	host := utils.StringWithFallback(svr.host, "localhost")

	var (
		certFile = ""
		keyFile  = ""
		scheme   = "http"
	)

	if svr.tls != nil {
		certFile = svr.tls.CertFile
		keyFile = svr.tls.KeyFile
	}

	if svr.tls != nil && certFile != "" && keyFile != "" {
		scheme = "https"
	}

	svr.logger.Info("Server is running", "URL",
		fmt.Sprintf("%s://%s:%d", scheme, host, svr.port))

	switch {
	case svr.tls != nil && certFile != "" && keyFile != "":
		err = svr.server.ListenAndServeTLS(certFile, keyFile)
	default:
		err = svr.server.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	if err != nil {
		return err
	}

	<-svr.idleConnsClosed

	svr.logger.Info("frontend closed")

	return
}

func (svr *Server) setupServer() {
	svr.server = &http.Server{Addr: svr.addr}
}

func (svr *Server) setupHandler() {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("Access-Control-Allow-Origin", "*")
				w.Header().Add("Access-Control-Allow-Methods", "*")
				w.Header().Add("Access-Control-Allow-Headers", "*")
				h.ServeHTTP(w, r)
			})
	})

	if svr.basicAuth != nil {
		r.Use(middleware.BasicAuth(
			"restricted",
			map[string]string{svr.basicAuth.Username: svr.basicAuth.Password},
		))
	}

	handlers.ConfigRoutes(r)

	r.Post("/shutdown", svr.handleShutdown)

	svr.server.Handler = r
}

func (svr *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	svr.logger.Info("received shutdown request")
	_, _ = w.Write([]byte("shutting down the Server...\n"))
	go func() {
		_ = svr.Shutdown(r.Context())
	}()
}
