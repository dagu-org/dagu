package test

import (
	"encoding/json"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/frontend"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/require"
)

type Server struct {
	Helper
}

func SetupServer(t *testing.T, opts ...HelperOption) Server {
	t.Helper()

	// Find an available port for the server to listen on.
	port := findPort(t)
	opts = append(opts, WithServerConfig(
		&config.Server{
			Host: "localhost",
			Port: port,
		},
	))

	helper := Setup(t, opts...)

	srv := Server{Helper: helper}
	go srv.runServer(t)

	// Wait a moment for the server to start.
	time.Sleep(500 * time.Millisecond)

	return srv
}

func (srv *Server) runServer(t *testing.T) {
	t.Helper()

	server := frontend.New(srv.Config, srv.Helper.Client)
	err := server.Serve(srv.Helper.Context)
	require.NoError(t, err, "failed to start server")
}

func (srv *Server) Client() Client {
	return Client{
		Server: *srv,
		client: resty.New(),
	}
}

type Client struct {
	Server
	client *resty.Client
}

func (c Client) Get(t *testing.T, path string, expectedStatus int) Response {
	t.Helper()

	req := c.client.R()
	url := "http://" + c.Server.Config.Server.Host + ":" + strconv.Itoa(c.Server.Config.Server.Port) + path
	res, err := req.Get(url)
	require.NoError(t, err, "failed to make GET request")
	require.Equal(t, expectedStatus, res.StatusCode(), "unexpected status code")

	return Response{
		Body:     string(res.Body()),
		Response: res,
	}
}

type Response struct {
	Body     string
	Response *resty.Response
}

func (r Response) Unmarshal(t *testing.T, v any) {
	t.Helper()
	err := json.Unmarshal([]byte(r.Body), v)
	require.NoError(t, err, "failed to unmarshal response body")
}

// findPort finds an available port.
func findPort(t *testing.T) int {
	t.Helper()
	tcpListener, err := net.Listen("tcp", ":0") // nolint:gosec
	require.NoError(t, err)
	port := tcpListener.Addr().(*net.TCPAddr).Port
	_ = tcpListener.Close()
	return port
}
