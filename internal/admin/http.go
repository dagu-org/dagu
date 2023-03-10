package admin

import (
	"context"
	"log"
	"net"
	"net/http"

	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/utils"
)

type server struct {
	config          *config.Config
	addr            string
	server          *http.Server
	admin           *adminHandler
	idleConnsClosed chan struct{}
}

func NewServer(cfg *config.Config) *server {
	return &server{
		addr:            net.JoinHostPort(cfg.Host, cfg.Port),
		config:          cfg,
		admin:           newAdminHandler(cfg, defaultRoutes(cfg)),
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

func (svr *server) Serve() (err error) {
	svr.setupServer()
	svr.setupHandler()

	svr.idleConnsClosed = make(chan struct{})

	host := utils.StringWithFallback(svr.config.Host, "localhost")
	log.Printf("admin server is running at \"http://%s:%s\"\n",
		host, svr.config.Port)

	err = svr.server.ListenAndServe()
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
	svr.server = &http.Server{
		Addr: svr.addr,
	}
}

func (svr *server) setupHandler() {
	svr.admin.addRoute(http.MethodPost, `^/shutdown$`, svr.handleShutdown)
	handler := requestLogger(svr.admin)
	handler = cors(handler)
	if svr.config.IsBasicAuth {
		handler = basicAuth(handler,
			svr.config.BasicAuthUsername,
			svr.config.BasicAuthPassword)
	}
	svr.server.Handler = handler
}

func (svr *server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	log.Println("received shutdown request")
	_, _ = w.Write([]byte("shutting down the dagu server...\n"))
	go func() {
		svr.Shutdown()
	}()
}
