package digraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthConfig(t *testing.T) {
	t.Run("AuthConfigFields", func(t *testing.T) {
		auth := &AuthConfig{
			Username: "test-user",
			Password: "test-pass",
			Auth:     "dGVzdC11c2VyOnRlc3QtcGFzcw==", // base64("test-user:test-pass")
		}

		assert.Equal(t, "test-user", auth.Username)
		assert.Equal(t, "test-pass", auth.Password)
		assert.Equal(t, "dGVzdC11c2VyOnRlc3QtcGFzcw==", auth.Auth)
	})
}

func TestDAGRegistryAuths(t *testing.T) {
	t.Run("DAGWithRegistryAuths", func(t *testing.T) {
		dag := &DAG{
			Name: "test-dag",
			RegistryAuths: map[string]*AuthConfig{
				"docker.io": {
					Username: "docker-user",
					Password: "docker-pass",
				},
				"ghcr.io": {
					Username: "github-user",
					Password: "github-token",
				},
			},
		}

		assert.NotNil(t, dag.RegistryAuths)
		assert.Len(t, dag.RegistryAuths, 2)

		dockerAuth, exists := dag.RegistryAuths["docker.io"]
		assert.True(t, exists)
		assert.Equal(t, "docker-user", dockerAuth.Username)

		ghcrAuth, exists := dag.RegistryAuths["ghcr.io"]
		assert.True(t, exists)
		assert.Equal(t, "github-user", ghcrAuth.Username)
	})
}
