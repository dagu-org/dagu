package remotenode

import (
	"time"

	"github.com/google/uuid"
)

// AuthMode represents the authentication method for a remote node.
type AuthMode string

const (
	AuthModeNone  AuthMode = "none"
	AuthModeBasic AuthMode = "basic"
	AuthModeToken AuthMode = "token"
)

// RemoteNode is the domain model for a managed remote node.
type RemoteNode struct {
	ID                string
	Name              string
	Description       string
	APIBaseURL        string
	AuthMode          AuthMode
	BasicAuthUsername string
	BasicAuthPassword string // plaintext in memory, encrypted at rest
	AuthToken         string // plaintext in memory, encrypted at rest
	SkipTLSVerify     bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NewRemoteNode creates a RemoteNode with a new UUID and current timestamps.
func NewRemoteNode(name, description, apiBaseURL string, authMode AuthMode) *RemoteNode {
	now := time.Now().UTC()
	return &RemoteNode{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		APIBaseURL:  apiBaseURL,
		AuthMode:    authMode,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// RemoteNodeForStorage is used for JSON serialization to persistent storage.
// Credential fields are encrypted at rest.
type RemoteNodeForStorage struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	Description          string    `json:"description,omitempty"`
	APIBaseURL           string    `json:"api_base_url"`
	AuthMode             AuthMode  `json:"auth_mode"`
	BasicAuthUsername    string    `json:"basic_auth_username,omitempty"`
	BasicAuthPasswordEnc string    `json:"basic_auth_password_enc,omitempty"`
	AuthTokenEnc         string    `json:"auth_token_enc,omitempty"`
	SkipTLSVerify        bool      `json:"skip_tls_verify,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// ToStorage converts a RemoteNode to RemoteNodeForStorage.
// Note: Credential encryption must be handled by the caller before setting
// BasicAuthPasswordEnc and AuthTokenEnc.
func (n *RemoteNode) ToStorage() *RemoteNodeForStorage {
	return &RemoteNodeForStorage{
		ID:                n.ID,
		Name:              n.Name,
		Description:       n.Description,
		APIBaseURL:        n.APIBaseURL,
		AuthMode:          n.AuthMode,
		BasicAuthUsername: n.BasicAuthUsername,
		SkipTLSVerify:     n.SkipTLSVerify,
		CreatedAt:         n.CreatedAt,
		UpdatedAt:         n.UpdatedAt,
	}
}

// ToRemoteNode converts RemoteNodeForStorage back to RemoteNode.
// Note: Credential decryption must be handled by the caller to populate
// BasicAuthPassword and AuthToken.
func (s *RemoteNodeForStorage) ToRemoteNode() *RemoteNode {
	return &RemoteNode{
		ID:                s.ID,
		Name:              s.Name,
		Description:       s.Description,
		APIBaseURL:        s.APIBaseURL,
		AuthMode:          s.AuthMode,
		BasicAuthUsername: s.BasicAuthUsername,
		SkipTLSVerify:     s.SkipTLSVerify,
		CreatedAt:         s.CreatedAt,
		UpdatedAt:         s.UpdatedAt,
	}
}
