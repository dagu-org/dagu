package remotenode

import (
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/stretchr/testify/assert"
)

func TestToConfigNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *RemoteNode
		expected config.RemoteNode
	}{
		{
			name: "basic auth with description",
			input: &RemoteNode{
				Name:              "node1",
				Description:       "Test node",
				APIBaseURL:        "http://example.com/api",
				AuthType:          AuthTypeBasic,
				BasicAuthUsername: "user",
				BasicAuthPassword: "pass",
				SkipTLSVerify:     true,
			},
			expected: config.RemoteNode{
				Name:              "node1",
				Description:       "Test node",
				APIBaseURL:        "http://example.com/api",
				IsBasicAuth:       true,
				BasicAuthUsername: "user",
				BasicAuthPassword: "pass",
				SkipTLSVerify:     true,
			},
		},
		{
			name: "token auth without description",
			input: &RemoteNode{
				Name:       "node2",
				APIBaseURL: "http://example.com/api",
				AuthType:   AuthTypeToken,
				AuthToken:  "tok-123",
			},
			expected: config.RemoteNode{
				Name:        "node2",
				APIBaseURL:  "http://example.com/api",
				IsAuthToken: true,
				AuthToken:   "tok-123",
			},
		},
		{
			name: "no auth",
			input: &RemoteNode{
				Name:       "node3",
				APIBaseURL: "http://example.com/api",
				AuthType:   AuthTypeNone,
			},
			expected: config.RemoteNode{
				Name:       "node3",
				APIBaseURL: "http://example.com/api",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ToConfigNode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFromConfigNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    config.RemoteNode
		expected *RemoteNode
	}{
		{
			name: "basic auth with description",
			input: config.RemoteNode{
				Name:              "node1",
				Description:       "Test node",
				APIBaseURL:        "http://example.com/api",
				IsBasicAuth:       true,
				BasicAuthUsername: "user",
				BasicAuthPassword: "pass",
				SkipTLSVerify:     true,
			},
			expected: &RemoteNode{
				ID:                "cfg:node1",
				Name:              "node1",
				Description:       "Test node",
				APIBaseURL:        "http://example.com/api",
				AuthType:          AuthTypeBasic,
				BasicAuthUsername: "user",
				BasicAuthPassword: "pass",
				SkipTLSVerify:     true,
			},
		},
		{
			name: "token auth without description",
			input: config.RemoteNode{
				Name:        "node2",
				APIBaseURL:  "http://example.com/api",
				IsAuthToken: true,
				AuthToken:   "tok-123",
			},
			expected: &RemoteNode{
				ID:         "cfg:node2",
				Name:       "node2",
				APIBaseURL: "http://example.com/api",
				AuthType:   AuthTypeToken,
				AuthToken:  "tok-123",
			},
		},
		{
			name: "no auth",
			input: config.RemoteNode{
				Name:       "node3",
				APIBaseURL: "http://example.com/api",
			},
			expected: &RemoteNode{
				ID:         "cfg:node3",
				Name:       "node3",
				APIBaseURL: "http://example.com/api",
				AuthType:   AuthTypeNone,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := FromConfigNode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRoundTripConfigNode(t *testing.T) {
	t.Parallel()

	original := &RemoteNode{
		Name:              "roundtrip",
		Description:       "Round-trip test",
		APIBaseURL:        "http://example.com/api",
		AuthType:          AuthTypeBasic,
		BasicAuthUsername: "user",
		BasicAuthPassword: "pass",
		SkipTLSVerify:     true,
	}

	cn := ToConfigNode(original)
	result := FromConfigNode(cn)

	assert.Equal(t, original.Name, result.Name)
	assert.Equal(t, original.Description, result.Description)
	assert.Equal(t, original.APIBaseURL, result.APIBaseURL)
	assert.Equal(t, original.AuthType, result.AuthType)
	assert.Equal(t, original.BasicAuthUsername, result.BasicAuthUsername)
	assert.Equal(t, original.BasicAuthPassword, result.BasicAuthPassword)
	assert.Equal(t, original.SkipTLSVerify, result.SkipTLSVerify)
}
