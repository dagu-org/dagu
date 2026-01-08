package gha

import "github.com/dagu-org/dagu/internal/core"

// ConfigSchema defines the schema for gha (GitHub Actions) executor config.
// This struct is ONLY for generating JSON Schema - not used at runtime.
type ConfigSchema struct {
	Runner           string            `json:"runner,omitempty" jsonschema:"Docker image to use as runner"`
	AutoRemove       bool              `json:"autoRemove,omitempty" jsonschema:"Automatically remove containers after execution"`
	Network          string            `json:"network,omitempty" jsonschema:"Docker network mode"`
	GithubInstance   string            `json:"githubInstance,omitempty" jsonschema:"GitHub instance for action resolution"`
	DockerSocket     string            `json:"dockerSocket,omitempty" jsonschema:"Custom Docker socket path"`
	Artifacts        *ArtifactsSchema  `json:"artifacts,omitempty" jsonschema:"Artifact server configuration"`
	ReuseContainers  bool              `json:"reuseContainers,omitempty" jsonschema:"Reuse containers between runs"`
	ForceRebuild     bool              `json:"forceRebuild,omitempty" jsonschema:"Force rebuild of action images"`
	ContainerOptions string            `json:"containerOptions,omitempty" jsonschema:"Additional Docker run options"`
	Privileged       bool              `json:"privileged,omitempty" jsonschema:"Run containers in privileged mode"`
	Capabilities     *CapabilitiesSchema `json:"capabilities,omitempty" jsonschema:"Linux capabilities configuration"`
}

// ArtifactsSchema defines the schema for artifact server configuration.
type ArtifactsSchema struct {
	Path string `json:"path,omitempty" jsonschema:"Artifact server path"`
	Port string `json:"port,omitempty" jsonschema:"Artifact server port"`
}

// CapabilitiesSchema defines the schema for Linux capabilities configuration.
type CapabilitiesSchema struct {
	Add  []string `json:"add,omitempty" jsonschema:"Capabilities to add"`
	Drop []string `json:"drop,omitempty" jsonschema:"Capabilities to drop"`
}

func init() {
	// Register for all executor type aliases
	core.RegisterExecutorConfigType[ConfigSchema]("github_action")
	core.RegisterExecutorConfigType[ConfigSchema]("github-action")
	core.RegisterExecutorConfigType[ConfigSchema]("gha")
}
