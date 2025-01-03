package sock

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/dagu-org/dagu/internal/logger"
)

var ErrServerRequestedShutdown = errors.New(
	"socket frontend is requested to shutdown",
)

// Server is a unix socket frontend that passes http requests to HandlerFunc.
type Server struct {
	addr        string
	handlerFunc HTTPHandlerFunc
	listener    net.Listener
	quit        atomic.Bool
	mu          sync.Mutex
}

// HTTPHandlerFunc is a function that handles HTTP requests.
type HTTPHandlerFunc func(w http.ResponseWriter, r *http.Request)

// NewServer creates a new unix socket frontend.
func NewServer(
	addr string,
	handlerFunc HTTPHandlerFunc,
) (*Server, error) {
	return &Server{
		addr:        addr,
		handlerFunc: handlerFunc,
	}, nil
}

// Serve starts listening and serving requests.
func (srv *Server) Serve(ctx context.Context, listen chan error) error {
	_ = os.Remove(srv.addr)
	var err error
	srv.listener, err = net.Listen("unix", srv.addr)
	if err != nil {
		if listen != nil {
			listen <- err
		}
		return err
	}
	if listen != nil {
		listen <- err
	}
	logger.Debug(ctx, "Unix socket is listening", "addr", srv.addr)

	defer func() {
		_ = srv.Shutdown(ctx)
		_ = os.Remove(srv.addr)
	}()
	for {
		conn, err := srv.listener.Accept()
		if srv.quit.Load() {
			return ErrServerRequestedShutdown
		}
		if err == nil {
			go func() {
				request, err := http.ReadRequest(bufio.NewReader(conn))
				if err != nil {
					logger.Error(ctx, "read request", "err", err)
				} else {
					srv.handlerFunc(newHTTPResponseWriter(&conn), request)
				}
				_ = conn.Close()
			}()
		}
	}
}

// Shutdown stops the frontend.
func (srv *Server) Shutdown(ctx context.Context) error {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if !srv.quit.Load() {
		srv.quit.Store(true)
		if srv.listener != nil {
			err := srv.listener.Close()
			if err != nil && !errors.Is(err, os.ErrClosed) {
				logger.Error(ctx, "close listener", "err", err)
			}
			return err
		}
	}
	return nil
}

var _ http.ResponseWriter = (*httpResponseWriter)(nil)

type httpResponseWriter struct {
	conn       *net.Conn
	header     http.Header
	statusCode int
}

func newHTTPResponseWriter(conn *net.Conn) http.ResponseWriter {
	return &httpResponseWriter{
		conn:       conn,
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}
}

func (w *httpResponseWriter) Write(data []byte) (int, error) {
	response := http.Response{
		StatusCode: w.statusCode,
		ProtoMajor: 1,
		ProtoMinor: 0,
		Body:       io.NopCloser(strings.NewReader(string(data))),
		Header:     w.header,
	}
	_ = response.Write(*w.conn)
	return 0, nil
}

func (w *httpResponseWriter) Header() http.Header {
	return w.header
}

func (w *httpResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}
