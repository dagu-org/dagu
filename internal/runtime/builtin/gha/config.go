package gha

import (
	"github.com/dagu-org/dagu/internal/core"
	"github.com/google/jsonschema-go/jsonschema"
)

var configSchema = &jsonschema.Schema{
	Type: "object",
	Properties: map[string]*jsonschema.Schema{
		"runner":            {Type: "string", Description: "Docker image to use as runner"},
		"auto_remove":       {Type: "boolean", Description: "Automatically remove containers after execution"},
		"network":           {Type: "string", Description: "Docker network mode"},
		"github_instance":   {Type: "string", Description: "GitHub instance for action resolution"},
		"docker_socket":     {Type: "string", Description: "Custom Docker socket path"},
		"reuse_containers":  {Type: "boolean", Description: "Reuse containers between runs"},
		"force_rebuild":     {Type: "boolean", Description: "Force rebuild of action images"},
		"container_options": {Type: "string", Description: "Additional Docker run options"},
		"privileged":        {Type: "boolean", Description: "Run containers in privileged mode"},
		"artifacts": {
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"path": {Type: "string", Description: "Artifact server path"},
				"port": {Type: "string", Description: "Artifact server port"},
			},
			Description: "Artifact server configuration",
		},
		"capabilities": {
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"add":  {Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "Capabilities to add"},
				"drop": {Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "Capabilities to drop"},
			},
			Description: "Linux capabilities configuration",
		},
	},
}

func init() {
	core.RegisterExecutorConfigSchema("github_action", configSchema)
	core.RegisterExecutorConfigSchema("github-action", configSchema)
	core.RegisterExecutorConfigSchema("gha", configSchema)
}
