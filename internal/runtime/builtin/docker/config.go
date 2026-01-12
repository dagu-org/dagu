package docker

import (
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/jsonschema-go/jsonschema"
)

// Config holds the configuration for creating or using a container.
type Config struct {
	// Image is the Docker image to use for creating a new container.
	Image string
	// Platform is the target platform for the container (e.g., linux/amd64).
	Platform string
	// ContainerName is the name or ID of an existing container to exec into.
	ContainerName string
	// Pull is the image pull policy for new containers.
	Pull core.PullPolicy
	// Container is the container configuration for new containers.
	// See https://pkg.go.dev/github.com/docker/docker/api/types/container#Config
	Container *container.Config
	// Host is the host configuration for new containers.
	// See https://pkg.go.dev/github.com/docker/docker/api/types/container#HostConfig
	Host *container.HostConfig
	// Network is the network configuration for new containers.
	// See https://pkg.go.dev/github.com/docker/docker@v27.5.1+incompatible/api/types/network#NetworkingConfig
	Network *network.NetworkingConfig
	// ExecOptions are the options for executing a command in the container.
	// See https://pkg.go.dev/github.com/docker/docker/api/types/container#ExecOptions
	ExecOptions *container.ExecOptions
	// AutoRemove indicates whether to automatically remove the container after it exits.
	AutoRemove bool
	// AuthManager is responsible for managing registry authentication.
	AuthManager *RegistryAuthManager

	// Startup mode for DAG-level container: "keepalive" (default) | "entrypoint" | "command"
	Startup string
	// WaitFor readiness gate: "running" (default) | "healthy"
	WaitFor string
	// StartCmd command for startup when startup == "command"
	StartCmd []string
	// LogPattern optional regex to wait for in logs before proceeding (if empty, no wait)
	LogPattern string
	// ShouldStart indicates whether the container should be started (for DAG-level containers)
	ShouldStart bool
}

// LoadConfig parses executorConfig into Container struct with registry auth
func LoadConfigFromMap(data map[string]any, registryAuths map[string]*core.AuthConfig) (*Config, error) {
	ret := struct {
		Container     container.Config         `mapstructure:"container"`
		Host          container.HostConfig     `mapstructure:"host"`
		Network       network.NetworkingConfig `mapstructure:"network"`
		Exec          container.ExecOptions    `mapstructure:"exec"`
		AutoRemove    any                      `mapstructure:"autoRemove"`
		Pull          any                      `mapstructure:"pull"`
		Platform      string                   `mapstructure:"platform"`
		ContainerName string                   `mapstructure:"containerName"`
		Image         string                   `mapstructure:"image"`
		// User-friendly shortcuts (mapped to nested fields)
		WorkingDir string   `mapstructure:"workingDir"`
		Volumes    []string `mapstructure:"volumes"`
	}{}

	md, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &ret,
		WeaklyTypedInput: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}
	if err := md.Decode(data); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	var autoRemove bool
	if ret.Host.AutoRemove {
		ret.Host.AutoRemove = false // Prevent removal by sdk
		autoRemove = true
	}

	pull := core.PullPolicyMissing
	if ret.Pull != nil {
		parsed, err := core.ParsePullPolicy(ret.Pull)
		if err != nil {
			return nil, err
		}
		pull = parsed
	}

	if ret.ContainerName == "" && ret.Image == "" {
		return nil, ErrImageOrContainerShouldNotBeEmpty
	}

	// Extract original presence of keys to drive validation (avoid zero-value ambiguity)
	hasKey := func(k string) bool {
		_, ok := data[k]
		return ok
	}

	nonEmptyMap := func(v any) bool {
		if v == nil {
			return false
		}
		if m, ok := v.(map[string]any); ok {
			return len(m) > 0
		}
		return true // present and not a map or nil
	}

	// Determine mode-affecting flags based on input
	hasImage := strings.TrimSpace(ret.Image) != ""
	hasContainerName := strings.TrimSpace(ret.ContainerName) != ""
	hasExec := hasKey("exec") && nonEmptyMap(data["exec"])

	// Validation rules:
	// - exec options are only valid with containerName (exec-in-existing mode)
	if hasImage && hasExec && !hasContainerName {
		return nil, ErrExecOnlyWithContainerName
	}

	if ret.AutoRemove != nil {
		v, err := stringutil.ParseBool(ret.AutoRemove)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate autoRemove value: %w", err)
		}
		autoRemove = v
	}

	// Apply user-friendly shortcuts to nested fields
	// workingDir -> container.WorkingDir (only if container.WorkingDir is not already set)
	if ret.WorkingDir != "" && ret.Container.WorkingDir == "" {
		ret.Container.WorkingDir = ret.WorkingDir
	}

	// volumes -> host.Binds (append to existing binds)
	if len(ret.Volumes) > 0 {
		ret.Host.Binds = append(ret.Host.Binds, ret.Volumes...)
	}

	// Set up registry authentication if provided
	var authManager *RegistryAuthManager
	if len(registryAuths) > 0 {
		authManager = NewRegistryAuthManager(registryAuths)
	}

	return loadDefaults(&Config{
		Image:         strings.TrimSpace(ret.Image),
		Platform:      strings.TrimSpace(ret.Platform),
		ContainerName: strings.TrimSpace(ret.ContainerName),
		Pull:          pull,
		Container:     &ret.Container,
		Host:          &ret.Host,
		Network:       &ret.Network,
		ExecOptions:   &ret.Exec,
		AutoRemove:    autoRemove,
		AuthManager:   authManager,
	}), nil
}

// NewFromContainerConfigWithAuth parses core.Container into Container struct with registry auth
func LoadConfig(workDir string, ct core.Container, registryAuths map[string]*core.AuthConfig) (*Config, error) {
	// Handle exec mode (exec into existing container)
	if ct.IsExecMode() {
		execOpts := &container.ExecOptions{
			User:       ct.User,
			WorkingDir: ct.GetWorkingDir(),
			Env:        ct.Env,
		}
		return loadDefaults(&Config{
			ContainerName: ct.Exec,
			ExecOptions:   execOpts,
		}), nil
	}

	// Validate required fields for image mode
	if ct.Image == "" {
		return nil, ErrImageRequired
	}

	// Initialize Docker configuration structs
	containerConfig := &container.Config{
		Image:      ct.Image,
		Env:        ct.Env,
		User:       ct.User,
		WorkingDir: ct.GetWorkingDir(),
	}

	// Convert healthcheck if provided
	if ct.Healthcheck != nil {
		containerConfig.Healthcheck = &container.HealthConfig{
			Test:        ct.Healthcheck.Test,
			Interval:    ct.Healthcheck.Interval,
			Timeout:     ct.Healthcheck.Timeout,
			StartPeriod: ct.Healthcheck.StartPeriod,
			Retries:     ct.Healthcheck.Retries,
		}
	}

	hostConfig := &container.HostConfig{}
	networkConfig := &network.NetworkingConfig{}
	execOptions := &container.ExecOptions{}

	// Parse volumes
	if len(ct.Volumes) > 0 {
		binds, mounts, err := parseVolumes(workDir, ct.Volumes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse volumes: %w", err)
		}
		if len(binds) > 0 {
			hostConfig.Binds = binds
		}
		if len(mounts) > 0 {
			hostConfig.Mounts = mounts
		}
	}

	// Parse ports
	if len(ct.Ports) > 0 {
		exposedPorts, portBindings, err := parsePorts(ct.Ports)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ports: %w", err)
		}
		containerConfig.ExposedPorts = exposedPorts
		hostConfig.PortBindings = portBindings
	}

	// Parse network mode
	if ct.Network != "" {
		networkMode := parseNetworkMode(ct.Network)
		hostConfig.NetworkMode = networkMode

		// If it's a custom network, add it to the endpoints config
		if !isStandardNetworkMode(ct.Network) {
			networkConfig.EndpointsConfig = map[string]*network.EndpointSettings{
				ct.Network: {},
			}
		}
	}

	// autoRemove is the inverse of KeepContainer
	autoRemove := !ct.KeepContainer

	// Apply restart policy if specified
	if ct.RestartPolicy != "" {
		rp, err := parseRestartPolicy(ct.RestartPolicy)
		if err != nil {
			return nil, fmt.Errorf("failed to parse restartPolicy: %w", err)
		}
		hostConfig.RestartPolicy = rp
	}

	// Set up registry authentication if provided
	var authManager *RegistryAuthManager
	if len(registryAuths) > 0 {
		authManager = NewRegistryAuthManager(registryAuths)
	}

	return loadDefaults(&Config{
		ContainerName: ct.Name,
		Image:         ct.Image,
		Platform:      ct.Platform,
		Pull:          ct.PullPolicy,
		AutoRemove:    autoRemove,
		Container:     containerConfig,
		Host:          hostConfig,
		Network:       networkConfig,
		ExecOptions:   execOptions,
		Startup:       strings.ToLower(strings.TrimSpace(string(ct.Startup))),
		WaitFor:       strings.ToLower(strings.TrimSpace(string(ct.WaitFor))),
		LogPattern:    ct.LogPattern,
		StartCmd:      append([]string{}, ct.Command...),
		AuthManager:   authManager,
	}), nil
}

func loadDefaults(cfg *Config) *Config {
	if cfg.Startup == "" {
		cfg.Startup = "keepalive"
	}
	if cfg.WaitFor == "" {
		cfg.WaitFor = "running"
	}
	return cfg
}

func init() {
	core.RegisterExecutorConfigSchema("docker", configSchema)
	core.RegisterExecutorConfigSchema("container", configSchema)
}

// configSchema defines the JSON schema for docker/container executor config.
// Validates that either image or containerName is provided, and that exec
// with image also requires containerName.
var configSchema = &jsonschema.Schema{
	Type: "object",
	Properties: map[string]*jsonschema.Schema{
		"image":         {Type: "string", Description: "Docker image (for new container mode)"},
		"containerName": {Type: "string", Description: "Container name (for exec mode or to name new container)"},
		"platform":      {Type: "string", Description: "Target platform (e.g., linux/amd64)"},
		"pull":          {Type: "string", Description: "Image pull policy (always, never, missing)"},
		"autoRemove":    {Type: "boolean", Description: "Remove container after exit"},
		"workingDir":    {Type: "string", Description: "Working directory inside container"},
		"volumes":       {Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "Volume bindings (host:container)"},
		"container":     {Type: "object", AdditionalProperties: &jsonschema.Schema{}},
		"host":          {Type: "object", AdditionalProperties: &jsonschema.Schema{}},
		"network":       {Type: "object", AdditionalProperties: &jsonschema.Schema{}},
		"exec":          {Type: "object", AdditionalProperties: &jsonschema.Schema{}},
	},
	AllOf: []*jsonschema.Schema{
		// Require at least one of image or containerName
		{
			AnyOf: []*jsonschema.Schema{
				{Required: []string{"image"}},
				{Required: []string{"containerName"}},
			},
		},
		// If exec + image, then containerName required
		{
			If:   &jsonschema.Schema{Required: []string{"exec", "image"}},
			Then: &jsonschema.Schema{Required: []string{"containerName"}},
		},
	},
}
