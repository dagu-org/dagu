package web

import (
	"context"
	"errors"
	"github.com/yohamta/dagu/service/frontend/web/handler"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/utils"
)

type Server struct {
	config          *config.Config
	addr            string
	server          *http.Server
	idleConnsClosed chan struct{}
}

func NewServer(cfg *config.Config) *Server {
	return &Server{
		addr:            net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		config:          cfg,
		idleConnsClosed: nil,
	}
}

func (svr *Server) Shutdown(_ context.Context) error {
	err := svr.server.Shutdown(context.Background())
	if err != nil {
		log.Printf("Server shutdown: %v", err)
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
	host := utils.StringWithFallback(svr.config.Host, "localhost")

	var (
		certFile = ""
		keyFile  = ""
		scheme   = "http"
	)

	if svr.config.TLS != nil {
		certFile = svr.config.TLS.CertFile
		keyFile = svr.config.TLS.KeyFile
	}

	if svr.config.TLS != nil && certFile != "" && keyFile != "" {
		scheme = "https"
	}

	log.Printf("Server is running at \"%s://%s:%d\"\n", scheme, host, svr.config.Port)

	switch {
	case svr.config.TLS != nil && certFile != "" && keyFile != "":
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

	log.Printf("frontend closed")

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

	if svr.config.IsBasicAuth {
		r.Use(middleware.BasicAuth(
			"restricted",
			map[string]string{
				svr.config.BasicAuthUsername: svr.config.BasicAuthPassword,
			},
		))
	}

	handlers.ConfigRoutes(r)

	r.Post("/shutdown", svr.handleShutdown)

	svr.server.Handler = r
}

func (svr *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	log.Println("received shutdown request")
	_, _ = w.Write([]byte("shutting down the Server...\n"))
	go func() {
		_ = svr.Shutdown(r.Context())
	}()
}
