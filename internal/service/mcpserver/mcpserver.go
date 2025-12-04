package mcpserver

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/dirlock"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPServer struct {
	server   *mcp.Server
	logDir   string
	running  atomic.Bool
	config   *config.Config
	dirLock  dirlock.DirLock
	location *time.Location
}

func New(cfg *config.Config) (*MCPServer, error) {

	timeLoc := cfg.Global.Location
	if timeLoc == nil {
		timeLoc = time.Local
	}
	lockOpts := &dirlock.LockOptions{
		StaleThreshold: cfg.Scheduler.LockStaleThreshold,
		RetryInterval:  cfg.Scheduler.LockRetryInterval,
	}
	lockDir := filepath.Join(cfg.Paths.DataDir, "mcpserver", "locks")
	dirLock := dirlock.New(lockDir, lockOpts)

	s := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)

	s.AddTool(&mcp.Tool{
		Name:        "list_DAGs",
		Description: "List all the Direct Acyclic Graphs in the server",
		InputSchema: &jsonschema.Schema{
			Type: "object",
		},
	}, listDags)

	/*
		s.AddTool(&mcp.Tool{
			Name:        "execute_DAG",
			Description: "Execute a specific Workflow",
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {Type: "string", MaxLength: jsonschema.Ptr(256)},
				},
			},
		}, executeDag)


		s.AddTool(&mcp.Tool{
			Name:        "create_DAG",
			Description: "Create a new workflow",
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {Type: "string", MaxLength: jsonschema.Ptr(10)},
				},
			},
		}, createDag)
	*/
	return &MCPServer{
		server:   s,
		logDir:   cfg.Paths.LogDir,
		location: timeLoc,
		dirLock:  dirLock,
	}, nil
}

func listDags(ctx context.Context, ctr *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// host := os.Getenv("DAGU_HOST")
	host := "localhost"
	// port := os.Getenv("DAGU_PORT")
	port := "8080"
	//api_base_url := os.Getenv("DAGU_API_BASE_URL")
	api_base_url := "api/v2"

	resp, err := http.Get(fmt.Sprintf("http://%s:%s/%s/dags", host, port, api_base_url))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(body)},
		},
		IsError: err != nil,
	}, nil
}

/*
	func executeDag(ctx context.Context, ctr *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// host := os.Getenv("DAGU_HOST")
		host := "localhost"
		// port := os.Getenv("DAGU_PORT")
		port := "8080"
		// api_base_url := os.Getenv("DAGU_API_BASE_URL")
		api_base_url := "api/v2"

		resp, err := http.Get(fmt.Sprintf("http://%s:%s/%s/dags", host, port, api_base_url))

		return &mcp.CallToolResult{
			StructuredContent: resp.Body,
			IsError:           err != nil,
		}, nil
	}

	func createDag(ctx context.Context, ctr *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// host := os.Getenv("DAGU_HOST")
		host := "localhost"
		// port := os.Getenv("DAGU_PORT")
		port := "8080"
		// api_base_url := os.Getenv("DAGU_API_BASE_URL")
		api_base_url := "api/v2"

		resp, err := http.Post(fmt.Sprintf("http://%s:%s/%s/dags", host, port, api_base_url))

		return &mcp.CallToolResult{
			StructuredContent: resp.Body,
			IsError:           err != nil,
		}, nil
	}
*/
func (s *MCPServer) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Acquire directory lock first to prevent multiple mcp server instances
	logger.Info(ctx, "Waiting to acquire mcp server lock")
	if err := s.dirLock.Lock(ctx); err != nil {
		return fmt.Errorf("failed to acquire mcp server lock: %w", err)
	}

	logger.Info(ctx, "Acquired mcp server lock")

	if err := s.server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		logger.Error(ctx, "MCP server failed", slog.String("error", err.Error()))
	}

	return nil
}
