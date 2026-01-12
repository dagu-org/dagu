package coordinator

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/core/exec"
)

// StaticRegistry is a simple service registry that returns a fixed list of coordinator addresses.
// This is useful for shared-nothing worker deployments where the coordinator addresses are known
// and specified via CLI flags or environment variables.
type StaticRegistry struct {
	coordinators []exec.HostInfo
}

// NewStaticRegistry creates a new StaticRegistry from a list of address strings.
// Each address should be in the format "host:port" (e.g., "coordinator-1:50055").
func NewStaticRegistry(addresses []string) (*StaticRegistry, error) {
	hosts := make([]exec.HostInfo, 0, len(addresses))

	for _, addr := range addresses {
		if addr == "" {
			continue
		}

		host, port, err := parseAddress(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid coordinator address %q: %w", addr, err)
		}

		hosts = append(hosts, exec.HostInfo{
			ID:        fmt.Sprintf("coord-%d", len(hosts)),
			Host:      host,
			Port:      port,
			Status:    exec.ServiceStatusActive,
			StartedAt: time.Now(),
		})
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("no valid coordinator addresses provided")
	}

	return &StaticRegistry{coordinators: hosts}, nil
}

// parseAddress parses a "host:port" address string.
func parseAddress(addr string) (host string, port int, err error) {
	// Handle addresses with or without port
	parts := strings.Split(addr, ":")
	if len(parts) > 2 {
		return "", 0, fmt.Errorf("invalid address format, expected host:port")
	}

	host = parts[0]
	if host == "" {
		return "", 0, fmt.Errorf("host cannot be empty")
	}

	// Default port for coordinator
	port = 50055
	if len(parts) == 2 {
		p, err := strconv.Atoi(parts[1])
		if err != nil {
			return "", 0, fmt.Errorf("invalid port: %w", err)
		}
		if p <= 0 || p > 65535 {
			return "", 0, fmt.Errorf("port must be between 1 and 65535")
		}
		port = p
	}

	return host, port, nil
}

// Register is a no-op for StaticRegistry since we don't support registration.
func (r *StaticRegistry) Register(_ context.Context, _ exec.ServiceName, _ exec.HostInfo) error {
	// Static registry doesn't support registration
	return nil
}

// Unregister is a no-op for StaticRegistry since we don't support registration.
func (r *StaticRegistry) Unregister(_ context.Context) {
	// Static registry doesn't support registration
}

// GetServiceMembers returns the list of coordinator hosts.
// Only ServiceNameCoordinator is supported; other services return an empty list.
func (r *StaticRegistry) GetServiceMembers(_ context.Context, name exec.ServiceName) ([]exec.HostInfo, error) {
	if name == exec.ServiceNameCoordinator {
		return r.coordinators, nil
	}
	// Return empty list for other services
	return nil, nil
}

// UpdateStatus is a no-op for StaticRegistry since we don't support status updates.
func (r *StaticRegistry) UpdateStatus(_ context.Context, _ exec.ServiceName, _ exec.ServiceStatus) error {
	// Static registry doesn't support status updates
	return nil
}

// Ensure StaticRegistry implements ServiceRegistry
var _ exec.ServiceRegistry = (*StaticRegistry)(nil)
