package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func New(ctx context.Context) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)

	s.AddTool(&mcp.Tool{
		Name: "List DAGs",
	}, func(ctx context.Context, ctr *mcp.CallToolRequest) (*mcp.CallToolResult, error) {})

	//s.AddTool(&mcp.Tool{
	// 	Name: "Execute DAG",
	//}, func(ctx context.Context, ctr *mcp.CallToolRequest) (*mcp.CallToolResult, error) {})

	//s.AddTool(&mcp.Tool{
	//	Name: "Create DAG",
	//}, func(ctx context.Context, ctr *mcp.CallToolRequest) (*mcp.CallToolResult, error) {})

	return s
}

func Start(ctx context.Context, s *mcp.Server) {
	s.Run(ctx)
}
