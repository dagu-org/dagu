package remotenode

import (
	"context"
	"log/slog"

	"github.com/dagu-org/dagu/internal/cmn/config"
)

// Source indicates where a remote node is defined.
type Source string

const (
	SourceConfig Source = "config"
	SourceStore  Source = "store"
)

// ResolvedNode wraps a remote node with its source information.
type ResolvedNode struct {
	*RemoteNode
	Source Source
}

// Resolver merges remote nodes from config file and store.
// Store nodes take precedence over config nodes on name collision.
type Resolver struct {
	configNodes map[string]config.RemoteNode
	store       Store
}

// NewResolver creates a Resolver from config-file nodes and an optional store.
func NewResolver(configNodes []config.RemoteNode, store Store) *Resolver {
	nodeMap := make(map[string]config.RemoteNode, len(configNodes))
	for _, n := range configNodes {
		nodeMap[n.Name] = n
	}
	return &Resolver{
		configNodes: nodeMap,
		store:       store,
	}
}

// GetByName looks up a remote node by name, trying the store first (if available),
// then falling back to config. Returns a config.RemoteNode so existing proxy/SSE
// code requires minimal changes.
func (r *Resolver) GetByName(ctx context.Context, name string) (*config.RemoteNode, error) {
	// Try store first (higher precedence)
	if r.store != nil {
		node, err := r.store.GetByName(ctx, name)
		if err == nil {
			cn := ToConfigNode(node)
			return &cn, nil
		}
		// Only fall through if not found; other errors are real failures
		if err != ErrRemoteNodeNotFound {
			return nil, err
		}
	}

	// Fall back to config
	cn, ok := r.configNodes[name]
	if !ok {
		return nil, ErrRemoteNodeNotFound
	}
	return &cn, nil
}

// GetByID retrieves a store-managed remote node by ID.
// Config nodes use synthetic IDs (cfg:<name>) and are resolved via GetByName.
func (r *Resolver) GetByID(ctx context.Context, id string) (*RemoteNode, error) {
	if r.store == nil {
		return nil, ErrRemoteNodeNotFound
	}
	return r.store.GetByID(ctx, id)
}

// ListAll returns all nodes from both sources.
// Store nodes override config nodes on name collision.
func (r *Resolver) ListAll(ctx context.Context) ([]ResolvedNode, error) {
	seen := make(map[string]struct{})
	var result []ResolvedNode

	// Store nodes first (higher precedence)
	if r.store != nil {
		storeNodes, err := r.store.List(ctx)
		if err != nil {
			slog.Warn("Failed to list store remote nodes", slog.String("error", err.Error()))
		} else {
			for _, n := range storeNodes {
				seen[n.Name] = struct{}{}
				result = append(result, ResolvedNode{
					RemoteNode: n,
					Source:     SourceStore,
				})
			}
		}
	}

	// Config nodes (skip if overridden by store)
	for _, cn := range r.configNodes {
		if _, ok := seen[cn.Name]; ok {
			continue
		}
		result = append(result, ResolvedNode{
			RemoteNode: FromConfigNode(cn),
			Source:     SourceConfig,
		})
	}

	return result, nil
}

// ListNames returns deduplicated node names from both sources.
func (r *Resolver) ListNames(ctx context.Context) ([]string, error) {
	nodes, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(nodes))
	for _, n := range nodes {
		names = append(names, n.Name)
	}
	return names, nil
}

// ConfigNodeIDPrefix is prepended to config node names to form a synthetic ID.
const ConfigNodeIDPrefix = "cfg:"

// ToConfigNode converts a domain RemoteNode to config.RemoteNode
// for backward compatibility with proxy/SSE middleware.
func ToConfigNode(n *RemoteNode) config.RemoteNode {
	return config.RemoteNode{
		Name:              n.Name,
		Description:       n.Description,
		APIBaseURL:        n.APIBaseURL,
		AuthMode:          string(n.AuthMode),
		BasicAuthUsername: n.BasicAuthUsername,
		BasicAuthPassword: n.BasicAuthPassword,
		AuthToken:         n.AuthToken,
		SkipTLSVerify:     n.SkipTLSVerify,
	}
}

// FromConfigNode converts a config.RemoteNode to a domain RemoteNode.
// Config nodes receive a synthetic ID of "cfg:<name>".
func FromConfigNode(cn config.RemoteNode) *RemoteNode {
	authMode := AuthMode(cn.AuthMode)
	if authMode == "" {
		authMode = AuthModeNone
	}
	return &RemoteNode{
		ID:                ConfigNodeIDPrefix + cn.Name,
		Name:              cn.Name,
		Description:       cn.Description,
		APIBaseURL:        cn.APIBaseURL,
		AuthMode:          authMode,
		BasicAuthUsername: cn.BasicAuthUsername,
		BasicAuthPassword: cn.BasicAuthPassword,
		AuthToken:         cn.AuthToken,
		SkipTLSVerify:     cn.SkipTLSVerify,
	}
}
