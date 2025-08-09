package models

import (
	"context"
)

// ServiceRegistry is responsible for registering and persisting
// running service information.
type ServiceRegistry interface {
	// Register registers services for the given service name and host info.
	// It returns an error if the registry failed to start heartbeat.
	Register(ctx context.Context, serviceName ServiceName, hostInfo HostInfo) error

	// Unregister un-registers current service.
	Unregister(ctx context.Context)

	// GetServiceMembers returns the list of active hosts for the given service.
	// This method combines service resolution and member discovery.
	GetServiceMembers(ctx context.Context, serviceName ServiceName) ([]HostInfo, error)
}

// ServiceName represents the name of a service in the service discovery system
type ServiceName string

const (
	// ServiceNameCoordinator is the name of the coordinator service
	ServiceNameCoordinator ServiceName = "coordinator"
	// ServiceNameScheduler is the name of the scheduler service
)

// HostInfo contains information about a host in the service discovery system
type HostInfo struct {
	// ID is a unique identifier for the host
	ID string
	// HostPort is the combined host and port in the format "host:port"
	HostPort string
}
