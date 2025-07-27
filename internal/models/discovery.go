package models

import (
	"context"
)

// ServiceMonitor monitors service availability
type ServiceMonitor interface {
	// Start begins monitoring services and registers this instance.
	// It should return an error if the monitor cannot start.
	Start(ctx context.Context, serviceName ServiceName, hostInfo HostInfo) error

	// Resolver returns the service resolver used by this monitor
	Resolver(ctx context.Context, serviceName ServiceName) ServiceResolver

	// Stop stops the service monitor. It should clean up any resources used.
	Stop(ctx context.Context)
}

// ServiceName represents the name of a service in the service discovery system
type ServiceName string

const (
	// ServiceNameCoordinator is the name of the coordinator service
	ServiceNameCoordinator ServiceName = "coordinator"
)

// ServiceResolver resolves service endpoints
type ServiceResolver interface {
	// Members returns the list of active hosts for the service.
	Members(ctx context.Context) ([]HostInfo, error)
}

// HostInfo contains information about a host in the service discovery system
type HostInfo struct {
	// ID is a unique identifier for the host
	ID string
	// HostPort is the combined host and port in the format "host:port"
	HostPort string
}
