package http

import (
	"context"
	"errors"
	"github.com/go-openapi/loads"
	flags "github.com/jessevdk/go-flags"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/internal/logger/tag"
	pkgapi "github.com/dagu-dev/dagu/service/frontend/http/api"
	"github.com/dagu-dev/dagu/service/frontend/http/handler"
	"github.com/dagu-dev/dagu/service/frontend/restapi"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	pkgmiddleware "github.com/dagu-dev/dagu/service/frontend/http/middleware"
	"github.com/dagu-dev/dagu/service/frontend/restapi/operations"

	"github.com/go-chi/chi/v5"
)

type BasicAuth struct {
	Username string
	Password string
}

type ServerParams struct {
	Host      string
	Port      int
	BasicAuth *BasicAuth
	TLS       *config.TLS
	Logger    logger.Logger
}

type Server struct {
	host      string
	port      int
	basicAuth *BasicAuth
	tls       *config.TLS
	logger    logger.Logger
	server    *restapi.Server
	//server    *http.Server
	//addr            string
	//idleConnsClosed chan struct{}
}

func NewServer(params ServerParams) *Server {
	return &Server{
		//addr: net.JoinHostPort(params.Host, strconv.Itoa(params.Port)),
		//idleConnsClosed: nil,
		host:      params.Host,
		port:      params.Port,
		basicAuth: params.BasicAuth,
		tls:       params.TLS,
		logger:    params.Logger,
	}
}

func (svr *Server) Shutdown() {
	if svr.server == nil {
		return
	}
	err := svr.server.Shutdown()
	if err != nil {
		svr.logger.Warn("Server shutdown", tag.Error(err))
	}
}

//func (svr *Server) Shutdown(_ context.Context) error {
//	err := svr.server.Shutdown(context.Background())
//	if err != nil {
//		svr.logger.Warn("Server shutdown", tag.Error(err))
//	}
//	if svr.idleConnsClosed != nil {
//		close(svr.idleConnsClosed)
//		svr.idleConnsClosed = nil
//	}
//	return nil
//}

//	func (svr *Server) Signal(_ os.Signal) {
//		_ = svr.Shutdown(context.Background())
//	}
func (svr *Server) Serve(ctx context.Context) (err error) {

	//var (
	//	certFile = ""
	//	keyFile  = ""
	//	scheme   = "http"
	//)
	//
	//if svr.tls != nil {
	//	certFile = svr.tls.CertFile
	//	keyFile = svr.tls.KeyFile
	//}
	//
	//if svr.tls != nil && certFile != "" && keyFile != "" {
	//	scheme = "https"
	//}
	//
	//svr.setupServer()
	//svr.setupHandler()
	//
	//svr.idleConnsClosed = make(chan struct{})
	//host := utils.StringWithFallback(svr.host, "localhost")
	//
	//svr.logger.Info("Server is running", "URL",
	//	fmt.Sprintf("%s://%s:%d", scheme, host, svr.port))
	//
	//switch {
	//case svr.tls != nil && certFile != "" && keyFile != "":
	//	err = svr.server.ListenAndServeTLS(certFile, keyFile)
	//default:
	//	err = svr.server.ListenAndServe()
	//}
	//if errors.Is(err, http.ErrServerClosed) {
	//	err = nil
	//}
	//if err != nil {
	//	return err
	//}
	//
	//<-svr.idleConnsClosed

	middlewareOptions := &pkgmiddleware.Options{
		Handler: handlers.ConfigRoutes(chi.NewRouter()),
	}
	if svr.basicAuth != nil {
		middlewareOptions.BasicAuth = &pkgmiddleware.BasicAuth{
			Username: svr.basicAuth.Username,
			Password: svr.basicAuth.Password,
		}
	}
	pkgmiddleware.Setup(middlewareOptions)

	swaggerSpec, err := loads.Analyzed(restapi.SwaggerJSON, "")
	if err != nil {
		svr.logger.Error("failed to load API spec", tag.Error(err))
		return err
	}

	api := operations.NewDaguAPI(swaggerSpec)
	pkgapi.Configure(api)

	svr.server = restapi.NewServer(api)
	defer svr.Shutdown()

	svr.server.Host = svr.host
	svr.server.Port = svr.port
	svr.server.ConfigureAPI()

	// Server run context
	serverCtx, serverStopCtx := context.WithCancel(ctx)

	// Listen for syscall signals for process to interrupt/quit
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig

		// Trigger graceful shutdown
		err := svr.server.Shutdown()
		if err != nil {
			svr.logger.Error("server shutdown error", tag.Error(err))
		}
		serverStopCtx()
	}()

	if svr.tls != nil {
		svr.server.TLSCertificate = flags.Filename(svr.tls.CertFile)
		svr.server.TLSCertificateKey = flags.Filename(svr.tls.KeyFile)
	}

	// Run the server
	err = svr.server.Serve()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		svr.logger.Error("server error", tag.Error(err))
	}

	// Wait for server context to be stopped
	<-serverCtx.Done()

	svr.logger.Info("server closed")

	return nil
}

//func (svr *Server) setupServer() {
//	svr.server = &http.Server{Addr: svr.addr}
//}
//
//func (svr *Server) setupHandler() {
//	r := chi.NewRouter()
//
//	r.Use(middleware.RequestID)
//	r.Use(middleware.Logger)
//	r.Use(middleware.Recoverer)
//
//	r.Use(func(h http.Handler) http.Handler {
//		return http.HandlerFunc(
//			func(w http.ResponseWriter, r *http.Request) {
//				w.Header().Add("Access-Control-Allow-Origin", "*")
//				w.Header().Add("Access-Control-Allow-Methods", "*")
//				w.Header().Add("Access-Control-Allow-Headers", "*")
//				h.ServeHTTP(w, r)
//			})
//	})
//
//	if svr.basicAuth != nil {
//		r.Use(middleware.BasicAuth(
//			"restricted",
//			map[string]string{svr.basicAuth.Username: svr.basicAuth.Password},
//		))
//	}
//
//	handlers.ConfigRoutes(r)
//
//	//r.Post("/shutdown", svr.handleShutdown)
//
//	svr.server.Handler = r
//}

//func (svr *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
//	svr.logger.Info("received shutdown request")
//	_, _ = w.Write([]byte("shutting down the Server...\n"))
//	go func() {
//		_ = svr.Shutdown(r.Context())
//	}()
//}
