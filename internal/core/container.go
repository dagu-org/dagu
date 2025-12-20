package core

import (
	"fmt"
	"strconv"
)

// Container defines the container configuration for the DAG.
type Container struct {
	// Name is the container name to use. If empty, Docker generates a random name.
	Name string `yaml:"name,omitempty"`
	// Image is the container image to use.
	Image string `yaml:"image,omitempty"`
	// PullPolicy is the policy to pull the image (e.g., "Always", "IfNotPresent").
	PullPolicy PullPolicy `yaml:"pullPolicy,omitempty"`
	// Env specifies environment variables for the container.
	Env []string `yaml:"env,omitempty"` // List of environment variables in "key=value" format
	// Volumes specifies the volumes to mount in the container.
	Volumes []string `yaml:"volumes,omitempty"` // Map of volume names to volume definitions
	// User is the user to run the container as.
	User string `yaml:"user,omitempty"` // User to run the container as
	// WorkingDir is the working directory inside the container.
	WorkingDir string `yaml:"workingDir,omitempty"` // Working directory inside the container
	// WorkDir is the working directory inside the container.
	// Deprecated: use workingDir instead
	WorkDir string `yaml:"workDir,omitempty"` // Working directory inside the container
	// Platform specifies the platform for the container (e.g., "linux/amd64").
	Platform string `yaml:"platform,omitempty"` // Platform for the container
	// Ports specifies the ports to expose from the container.
	Ports []string `yaml:"ports,omitempty"` // List of ports to expose
	// Network is the network configuration for the container.
	Network string `yaml:"network,omitempty"` // Network configuration for the container
	// KeepContainer is the flag to keep the container after the DAG run.
	KeepContainer bool `yaml:"keepContainer,omitempty"` // Keep the container after the DAG run
	// Startup determines how the DAG-level container starts up.
	// One of: "keepalive" (default), "entrypoint", "command".
	Startup ContainerStartup `yaml:"startup,omitempty"`
	// Command is used when Startup == "command".
	Command []string `yaml:"command,omitempty"`
	// WaitFor determines readiness gate before steps run: "running" (default) or "healthy".
	WaitFor ContainerWaitFor `yaml:"waitFor,omitempty"`
	// LogPattern optionally waits for a regex to appear in container logs before proceeding.
	LogPattern string `yaml:"logPattern,omitempty"`
	// RestartPolicy applies Docker restart policy for long-running containers ("no", "always", or "unless-stopped").
	RestartPolicy string `yaml:"restartPolicy,omitempty"`
}

// ContainerStartup is an enum for DAG-level container startup modes.
type ContainerStartup string

const (
	StartupKeepalive  ContainerStartup = "keepalive"
	StartupEntrypoint ContainerStartup = "entrypoint"
	StartupCommand    ContainerStartup = "command"
)

// ContainerWaitFor is an enum for container readiness conditions.
type ContainerWaitFor string

const (
	WaitForRunning ContainerWaitFor = "running"
	WaitForHealthy ContainerWaitFor = "healthy"
)

// GetWorkingDir returns workdir or working dir (backward compatibility)
func (ct Container) GetWorkingDir() string {
	if ct.WorkDir != "" {
		return ct.WorkDir
	}
	return ct.WorkingDir
}

// PullPolicy defines image pull policy for a container execution
type PullPolicy int

func (p PullPolicy) String() string {
	switch p {
	case PullPolicyAlways:
		return "always"
	case PullPolicyNever:
		return "never"
	case PullPolicyMissing:
		return "missing"
	default:
		return "unknown"
	}
}

const (
	PullPolicyAlways PullPolicy = iota
	PullPolicyNever
	PullPolicyMissing
)

var pullPolicyMap = map[string]PullPolicy{
	"always":  PullPolicyAlways,
	"missing": PullPolicyMissing,
	"never":   PullPolicyNever,
}

// boolToPullPolicy converts a boolean to a PullPolicy.
func boolToPullPolicy(b bool) PullPolicy {
	if b {
		return PullPolicyAlways
	}
	return PullPolicyNever
}

// ParsePullPolicy parses a pull policy from a raw value.
func ParsePullPolicy(raw any) (PullPolicy, error) {
	switch value := raw.(type) {
	case nil:
		// If the value is nil, return PullPolicyMissing
		return PullPolicyMissing, nil
	case string:
		if value == "" {
			// If the string is empty, return PullPolicyMissing
			return PullPolicyMissing, nil
		}
		// Try to parse the string as a pull policy
		pull, ok := pullPolicyMap[value]
		if ok {
			return pull, nil
		}

		// If the string is not a valid pull policy, try to parse it as a boolean
		b, err := strconv.ParseBool(value)
		if err != nil {
			return PullPolicyMissing, fmt.Errorf("failed to parse pull policy as boolean: %w", err)
		}
		return boolToPullPolicy(b), nil
	case bool:
		return boolToPullPolicy(value), nil
	default:
		return PullPolicyMissing, fmt.Errorf("invalid pull policy type: %T", raw)
	}
}
