package sock

import (
	"bufio"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

type Server struct {
	*Config
	listener net.Listener
	quit     bool
}

type Config struct {
	Addr        string
	HandlerFunc HttpHandlerFunc
}

type HttpHandlerFunc func(w http.ResponseWriter, r *http.Request)

func NewServer(c *Config) (*Server, error) {
	return &Server{
		Config: c,
		quit:   false,
	}, nil
}

var (
	ErrServerRequestedShutdown = errors.New("socket server is requested to shutdown")
)

func (svr *Server) Serve(listen chan error) error {
	os.Remove(svr.Addr)
	var err error
	svr.listener, err = net.Listen("unix", svr.Addr)
	if err != nil {
		listen <- err
		return err
	}
	listen <- err
	log.Printf("server is running at \"%v\"\n", svr.Addr)
	defer func() {
		svr.listener.Close()
		os.Remove(svr.Addr)
	}()
	for {
		conn, err := svr.listener.Accept()
		if svr.quit {
			return ErrServerRequestedShutdown
		}
		if err != nil {
			return err
		}
		go func() {
			request, err := http.ReadRequest(bufio.NewReader(conn))
			if err != nil {
				log.Printf("Failed to read request %v", err)
				return
			}
			svr.HandlerFunc(NewHttpResponseWriter(&conn), request)
			conn.Close()

			if svr.quit {
				svr.Shutdown()
			}
		}()
	}
}

func (svr *Server) Shutdown() {
	if !svr.quit {
		svr.quit = true
		if svr.listener != nil {
			if err := svr.listener.Close(); err != nil {
				log.Printf("failed to close listener: %s", err)
			}
		}
	}
}

type HttpResponseWriter struct {
	conn       *net.Conn
	header     http.Header
	statusCode int
}

func NewHttpResponseWriter(conn *net.Conn) http.ResponseWriter {
	return &HttpResponseWriter{
		conn:       conn,
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}
}

func (w *HttpResponseWriter) Write(data []byte) (int, error) {
	response := http.Response{
		StatusCode: w.statusCode,
		ProtoMajor: 1,
		ProtoMinor: 0,
		Body:       ioutil.NopCloser(strings.NewReader(string(data))),
		Header:     w.header,
	}
	response.Write(*w.conn)
	return 0, nil
}

func (w *HttpResponseWriter) Header() http.Header {
	return w.header
}

func (w *HttpResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}
