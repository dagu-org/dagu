package web

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/utils"
	"github.com/yohamta/dagu/internal/web/handlers"
)

type server struct {
	config          *config.Config
	addr            string
	server          *http.Server
	idleConnsClosed chan struct{}
}

func NewServer(cfg *config.Config) *server {
	return &server{
		addr:            net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		config:          cfg,
		idleConnsClosed: nil,
	}
}

func (svr *server) Shutdown() {
	err := svr.server.Shutdown(context.Background())
	if err != nil {
		log.Printf("server shutdown: %v", err)
	}
	if svr.idleConnsClosed != nil {
		close(svr.idleConnsClosed)
		svr.idleConnsClosed = nil
	}
}

func (svr *server) Signal(_ os.Signal) {
	svr.Shutdown()
}

func (svr *server) Serve() (err error) {
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

	log.Printf("server is running at \"%s://%s:%d\"\n", scheme, host, svr.config.Port)

	switch {
	case svr.config.TLS != nil && certFile != "" && keyFile != "":
		err = svr.server.ListenAndServeTLS(certFile, keyFile)
	default:
		err = svr.server.ListenAndServe()
	}
	if err == http.ErrServerClosed {
		err = nil
	}
	if err != nil {
		return err
	}

	<-svr.idleConnsClosed

	log.Printf("server closed")

	return
}

func (svr *server) setupServer() {
	svr.server = &http.Server{Addr: svr.addr}
}

func (svr *server) setupHandler() {
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

func (svr *server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	log.Println("received shutdown request")
	_, _ = w.Write([]byte("shutting down the dagu server...\n"))
	go func() {
		svr.Shutdown()
	}()
}
