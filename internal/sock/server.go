package sock

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/dagu-dev/dagu/internal/util"
)

// Server is a unix socket frontend that passes http requests to HandlerFunc.
type Server struct {
	*Config
	listener net.Listener
	quit     atomic.Bool
	mu       sync.Mutex
}

type Config struct {
	Addr        string
	HandlerFunc HttpHandlerFunc
}

// HttpHandlerFunc is a function that handles HTTP requests.
type HttpHandlerFunc func(w http.ResponseWriter, r *http.Request)

// NewServer creates a new unix socket frontend.
func NewServer(c *Config) (*Server, error) {
	return &Server{
		Config: c,
	}, nil
}

var (
	ErrServerRequestedShutdown = errors.New("socket frontend is requested to shutdown")
)

// Serve starts listening and serving requests.
func (svr *Server) Serve(listen chan error) error {
	_ = os.Remove(svr.Addr)
	var err error
	svr.listener, err = net.Listen("unix", svr.Addr)
	if err != nil {
		if listen != nil {
			listen <- err
		}
		return err
	}
	if listen != nil {
		listen <- err
	}
	log.Printf("frontend is running at \"%v\"\n", svr.Addr)
	defer func() {
		_ = svr.Shutdown()
		_ = os.Remove(svr.Addr)
	}()
	for {
		conn, err := svr.listener.Accept()
		if svr.quit.Load() {
			return ErrServerRequestedShutdown
		}
		if err == nil {
			go func() {
				request, err := http.ReadRequest(bufio.NewReader(conn))
				util.LogErr("read request", err)
				if err == nil {
					svr.HandlerFunc(newHttpResponseWriter(&conn), request)
				}
				_ = conn.Close()
			}()
		}
	}
}

// Shutdown stops the frontend.
func (svr *Server) Shutdown() error {
	svr.mu.Lock()
	defer svr.mu.Unlock()
	if !svr.quit.Load() {
		svr.quit.Store(true)
		if svr.listener != nil {
			err := svr.listener.Close()
			util.LogErr("close listener", err)
			return err
		}
	}
	return nil
}

type httpResponseWriter struct {
	conn       *net.Conn
	header     http.Header
	statusCode int
}

var _ http.ResponseWriter = (*httpResponseWriter)(nil)

func newHttpResponseWriter(conn *net.Conn) http.ResponseWriter {
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
