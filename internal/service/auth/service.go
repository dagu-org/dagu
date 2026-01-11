package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Service errors.
var (
	ErrInvalidCredentials   = errors.New("invalid username or password")
	ErrInvalidToken         = errors.New("invalid or expired token")
	ErrTokenExpired         = errors.New("token has expired")
	ErrMissingSecret        = errors.New("token secret is not configured")
	ErrPasswordMismatch     = errors.New("current password is incorrect")
	ErrWeakPassword         = errors.New("password does not meet requirements")
	ErrCannotDeleteSelf     = errors.New("cannot delete your own account")
	ErrInvalidAPIKey        = errors.New("invalid API key")
	ErrAPIKeyNotConfigured  = errors.New("API key management is not configured")
	ErrInvalidCreatorID     = errors.New("creator ID is required")
	ErrInvalidWebhookToken  = errors.New("invalid webhook token")
	ErrWebhookNotConfigured = errors.New("webhook management is not configured")
	ErrWebhookDisabled      = errors.New("webhook is disabled")
	ErrUserDisabled         = errors.New("your account has been disabled, contact administrator")
)

const (
	// defaultBcryptCost is the default cost for bcrypt hashing.
	defaultBcryptCost = 12
	// minPasswordLength is the minimum required password length.
	minPasswordLength = 8
	// defaultTokenTTL is the default token time-to-live.
	defaultTokenTTL = 24 * time.Hour
	// apiKeyPrefix is the prefix for all API keys.
	apiKeyPrefix = "dagu_"
	// apiKeyRandomBytes is the number of random bytes for API key generation.
	apiKeyRandomBytes = 32
	// apiKeyPrefixLength is the length of the key prefix stored for identification.
	apiKeyPrefixLength = 8
	// webhookTokenPrefix is the fixed prefix for all webhook tokens.
	webhookTokenPrefix = "dagu_wh_" //nolint:gosec // Not a credential, just a token prefix
	// webhookTokenRandomBytes is the number of random bytes for webhook token generation.
	webhookTokenRandomBytes = 32
	// webhookTokenPrefixLength is how many characters of the full token we persist.
	// Must be > len(webhookTokenPrefix) so the stored prefix includes random characters.
	webhookTokenPrefixLength = 12
)

// Config holds the configuration for the auth service.
type Config struct {
	// TokenSecret is the secret key for signing JWT tokens.
	TokenSecret string
	// TokenTTL is the token time-to-live.
	TokenTTL time.Duration
	// BcryptCost is the cost factor for bcrypt hashing.
	BcryptCost int
}

// Claims represents the JWT claims.
type Claims struct {
	jwt.RegisteredClaims
	UserID   string    `json:"uid"`
	Username string    `json:"username"`
	Role     auth.Role `json:"role"`
}

// Service provides authentication and user management functionality.
type Service struct {
	store        auth.UserStore
	apiKeyStore  auth.APIKeyStore
	webhookStore auth.WebhookStore
	config       Config
}

// Option is a functional option for configuring the Service.
type Option func(*Service)

// WithAPIKeyStore sets the API key store for the service.
func WithAPIKeyStore(store auth.APIKeyStore) Option {
	return func(s *Service) {
		s.apiKeyStore = store
	}
}

// WithWebhookStore sets the webhook store for the service.
func WithWebhookStore(store auth.WebhookStore) Option {
	return func(s *Service) {
		s.webhookStore = store
	}
}

// New creates a new auth service using the provided user store and configuration.
// If TokenTTL or BcryptCost are not set (<= 0) they are replaced with package defaults.
func New(store auth.UserStore, config Config, opts ...Option) *Service {
	if config.TokenTTL <= 0 {
		config.TokenTTL = defaultTokenTTL
	}
	if config.BcryptCost <= 0 {
		config.BcryptCost = defaultBcryptCost
	}
	svc := &Service{
		store:  store,
		config: config,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// dummyHash is a valid bcrypt hash used for timing attack prevention.
// When a user is not found, we still perform a bcrypt comparison against this
// hash to ensure consistent response times regardless of user existence.
var dummyHash = []byte("$2a$12$K8gHXqrFdFvMwJBG0VlJGuAGz3FwBmTm8xnNQblN2tCxrQgPLmwHa")

// Authenticate verifies credentials and returns the user if valid.
func (s *Service) Authenticate(ctx context.Context, username, password string) (*auth.User, error) {
	user, err := s.store.GetByUsername(ctx, username)
	if err != nil {
		// Always perform a bcrypt comparison to prevent timing attacks.
		// This ensures consistent response times regardless of error type.
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))

		if errors.Is(err, auth.ErrUserNotFound) || errors.Is(err, auth.ErrInvalidUsername) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Check if user is disabled
	if user.IsDisabled {
		return nil, ErrUserDisabled
	}

	return user, nil
}

// TokenResult contains the generated token and its expiry time.
type TokenResult struct {
	Token     string
	ExpiresAt time.Time
}

// GenerateToken creates a JWT token for the given user.
// Returns the token string and its expiry time.
func (s *Service) GenerateToken(user *auth.User) (*TokenResult, error) {
	if s.config.TokenSecret == "" {
		return nil, ErrMissingSecret
	}

	now := time.Now()
	expiresAt := now.Add(s.config.TokenTTL)
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(s.config.TokenSecret))
	if err != nil {
		return nil, err
	}

	return &TokenResult{
		Token:     signedToken,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateToken validates a JWT token and returns the claims.
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	if s.config.TokenSecret == "" {
		return nil, ErrMissingSecret
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.TokenSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// GetUserFromToken validates a token and returns the associated user.
func (s *Service) GetUserFromToken(ctx context.Context, tokenString string) (*auth.User, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	user, err := s.store.GetByID(ctx, claims.UserID)
	if err != nil {
		// If user was deleted after token was issued, treat as invalid token
		if errors.Is(err, auth.ErrUserNotFound) || errors.Is(err, auth.ErrInvalidUserID) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("failed to get user from token: %w", err)
	}

	// Check if user is disabled
	if user.IsDisabled {
		return nil, ErrUserDisabled
	}

	return user, nil
}

// CreateUserInput contains the input for creating a user.
type CreateUserInput struct {
	Username string
	Password string
	Role     auth.Role
}

// CreateUser creates a new user.
func (s *Service) CreateUser(ctx context.Context, input CreateUserInput) (*auth.User, error) {
	if err := s.validatePassword(input.Password); err != nil {
		return nil, err
	}

	if !input.Role.Valid() {
		return nil, fmt.Errorf("invalid role: %s", input.Role)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), s.config.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := auth.NewUser(input.Username, string(passwordHash), input.Role)
	if err := s.store.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// GetUser retrieves a user by ID.
func (s *Service) GetUser(ctx context.Context, id string) (*auth.User, error) {
	return s.store.GetByID(ctx, id)
}

// ListUsers returns all users.
func (s *Service) ListUsers(ctx context.Context) ([]*auth.User, error) {
	return s.store.List(ctx)
}

// UpdateUserInput contains the input for updating a user.
// Note: Password field is supported by the service for direct usage,
// but the API handler intentionally omits it - password changes should
// go through ChangePassword (user self-service) or ResetPassword (admin).
type UpdateUserInput struct {
	Username *string
	Role     *auth.Role
	Password *string
}

// UpdateUser updates an existing user.
func (s *Service) UpdateUser(ctx context.Context, id string, input UpdateUserInput) (*auth.User, error) {
	user, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Username != nil && *input.Username != "" {
		user.Username = *input.Username
	}

	if input.Role != nil {
		if !input.Role.Valid() {
			return nil, fmt.Errorf("invalid role: %s", *input.Role)
		}
		user.Role = *input.Role
	}

	if input.Password != nil && *input.Password != "" {
		if err := s.validatePassword(*input.Password); err != nil {
			return nil, err
		}
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), s.config.BcryptCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		user.PasswordHash = string(passwordHash)
	}

	user.UpdatedAt = time.Now().UTC()

	if err := s.store.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// DeleteUser deletes a user by ID.
// The currentUserID prevents users from deleting themselves.
func (s *Service) DeleteUser(ctx context.Context, id string, currentUserID string) error {
	if id == currentUserID {
		return ErrCannotDeleteSelf
	}
	return s.store.Delete(ctx, id)
}

// ChangePassword changes a user's password after verifying the old password.
func (s *Service) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	user, err := s.store.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrPasswordMismatch
	}

	// Validate new password
	if err := s.validatePassword(newPassword); err != nil {
		return err
	}

	// Hash new password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.config.BcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.PasswordHash = string(passwordHash)
	user.UpdatedAt = time.Now().UTC()

	return s.store.Update(ctx, user)
}

// ResetPassword allows an admin to reset a user's password without knowing the old password.
func (s *Service) ResetPassword(ctx context.Context, userID, newPassword string) error {
	user, err := s.store.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Validate new password
	if err := s.validatePassword(newPassword); err != nil {
		return err
	}

	// Hash new password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.config.BcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.PasswordHash = string(passwordHash)
	user.UpdatedAt = time.Now().UTC()

	return s.store.Update(ctx, user)
}

// EnsureAdminUser creates the admin user if no users exist.
// Returns the generated password if a new admin was created.
func (s *Service) EnsureAdminUser(ctx context.Context, username, password string) (string, bool, error) {
	count, err := s.store.Count(ctx)
	if err != nil {
		return "", false, fmt.Errorf("failed to count users: %w", err)
	}

	if count > 0 {
		return "", false, nil
	}

	// Generate password if not provided
	generatedPassword := password
	if generatedPassword == "" {
		generatedPassword, err = generateSecurePassword(16)
		if err != nil {
			return "", false, fmt.Errorf("failed to generate password: %w", err)
		}
	}

	_, err = s.CreateUser(ctx, CreateUserInput{
		Username: username,
		Password: generatedPassword,
		Role:     auth.RoleAdmin,
	})
	if err != nil {
		// Handle race condition: another process may have created the admin user
		if errors.Is(err, auth.ErrUserAlreadyExists) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to create admin user: %w", err)
	}

	return generatedPassword, true, nil
}

// validatePassword checks if a password meets the minimum requirements.
func (s *Service) validatePassword(password string) error {
	if len(password) < minPasswordLength {
		return fmt.Errorf("%w: minimum length is %d characters", ErrWeakPassword, minPasswordLength)
	}
	return nil
}

// generateSecurePassword returns a URL-safe base64-encoded string of the requested length
// built from cryptographically secure random bytes. It returns an error if a secure random
// source cannot be read.
func generateSecurePassword(length int) (string, error) {
	// Calculate required bytes to produce at least 'length' base64 characters.
	// Base64 encodes 3 bytes to 4 characters, so we need ceil(length * 3 / 4) bytes.
	requiredBytes := (length*3 + 3) / 4
	bytes := make([]byte, requiredBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// Use RawURLEncoding (no padding) and take exactly 'length' characters.
	encoded := base64.RawURLEncoding.EncodeToString(bytes)
	return encoded[:length], nil
}

// CreateAPIKeyInput contains the input for creating an API key.
type CreateAPIKeyInput struct {
	Name        string
	Description string
	Role        auth.Role
}

// CreateAPIKeyResult contains the result of creating an API key.
type CreateAPIKeyResult struct {
	APIKey  *auth.APIKey
	FullKey string // Only returned once at creation
}

// CreateAPIKey creates a new API key.
func (s *Service) CreateAPIKey(ctx context.Context, input CreateAPIKeyInput, creatorID string) (*CreateAPIKeyResult, error) {
	if s.apiKeyStore == nil {
		return nil, ErrAPIKeyNotConfigured
	}

	if input.Name == "" {
		return nil, auth.ErrInvalidAPIKeyName
	}

	if !input.Role.Valid() {
		return nil, fmt.Errorf("invalid role: %s", input.Role)
	}

	if creatorID == "" {
		return nil, ErrInvalidCreatorID
	}

	// Generate API key
	keyParts, err := generateAPIKey(s.config.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	apiKey, err := auth.NewAPIKey(input.Name, input.Description, input.Role, keyParts.keyHash, keyParts.keyPrefix, creatorID)
	if err != nil {
		return nil, err
	}
	if err := s.apiKeyStore.Create(ctx, apiKey); err != nil {
		return nil, err
	}

	return &CreateAPIKeyResult{
		APIKey:  apiKey,
		FullKey: keyParts.fullKey,
	}, nil
}

// apiKeyParts holds the components of a generated API key.
type apiKeyParts struct {
	fullKey   string
	keyPrefix string
	keyHash   string
}

// generateAPIKey generates a new API key with prefix, hash, and prefix for display.
func generateAPIKey(bcryptCost int) (*apiKeyParts, error) {
	randomBytes := make([]byte, apiKeyRandomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	encoded := stringutil.Base58Encode(randomBytes)
	fullKey := apiKeyPrefix + encoded

	// Store first N characters as prefix for identification
	var keyPrefix string
	if len(fullKey) >= apiKeyPrefixLength {
		keyPrefix = fullKey[:apiKeyPrefixLength]
	} else {
		keyPrefix = fullKey
	}

	// Hash the full key
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(fullKey), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash API key: %w", err)
	}

	return &apiKeyParts{
		fullKey:   fullKey,
		keyPrefix: keyPrefix,
		keyHash:   string(hashBytes),
	}, nil
}

// GetAPIKey retrieves an API key by ID.
func (s *Service) GetAPIKey(ctx context.Context, id string) (*auth.APIKey, error) {
	if s.apiKeyStore == nil {
		return nil, ErrAPIKeyNotConfigured
	}
	return s.apiKeyStore.GetByID(ctx, id)
}

// ListAPIKeys returns all API keys.
func (s *Service) ListAPIKeys(ctx context.Context) ([]*auth.APIKey, error) {
	if s.apiKeyStore == nil {
		return nil, ErrAPIKeyNotConfigured
	}
	return s.apiKeyStore.List(ctx)
}

// UpdateAPIKeyInput contains the input for updating an API key.
type UpdateAPIKeyInput struct {
	Name        *string
	Description *string
	Role        *auth.Role
}

// UpdateAPIKey updates an existing API key.
func (s *Service) UpdateAPIKey(ctx context.Context, id string, input UpdateAPIKeyInput) (*auth.APIKey, error) {
	if s.apiKeyStore == nil {
		return nil, ErrAPIKeyNotConfigured
	}

	apiKey, err := s.apiKeyStore.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil && *input.Name != "" {
		apiKey.Name = *input.Name
	}

	if input.Description != nil {
		apiKey.Description = *input.Description
	}

	if input.Role != nil {
		if !input.Role.Valid() {
			return nil, fmt.Errorf("invalid role: %s", *input.Role)
		}
		apiKey.Role = *input.Role
	}

	apiKey.UpdatedAt = time.Now().UTC()

	if err := s.apiKeyStore.Update(ctx, apiKey); err != nil {
		return nil, err
	}

	return apiKey, nil
}

// DeleteAPIKey deletes an API key by ID.
func (s *Service) DeleteAPIKey(ctx context.Context, id string) error {
	if s.apiKeyStore == nil {
		return ErrAPIKeyNotConfigured
	}
	return s.apiKeyStore.Delete(ctx, id)
}

// ValidateAPIKey validates an API key and returns the associated APIKey if valid.
// Note: O(n) bcrypt comparisons (~100ms each). Fine for <100 keys.
func (s *Service) ValidateAPIKey(ctx context.Context, keySecret string) (*auth.APIKey, error) {
	if s.apiKeyStore == nil {
		return nil, ErrAPIKeyNotConfigured
	}

	// Check for correct prefix
	if !strings.HasPrefix(keySecret, apiKeyPrefix) {
		return nil, ErrInvalidAPIKey
	}

	keys, err := s.apiKeyStore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	for _, key := range keys {
		if err := bcrypt.CompareHashAndPassword([]byte(key.KeyHash), []byte(keySecret)); err == nil {
			// Update last used timestamp synchronously.
			// This avoids goroutine leaks and race conditions with Delete.
			if err := s.apiKeyStore.UpdateLastUsed(ctx, key.ID); err != nil {
				slog.Error("failed to update API key last used timestamp", "keyID", key.ID, "error", err)
			}
			return key, nil
		}
	}

	return nil, ErrInvalidAPIKey
}

// HasAPIKeyStore returns true if API key management is configured.
func (s *Service) HasAPIKeyStore() bool {
	return s.apiKeyStore != nil
}

// HasWebhookStore returns true if webhook management is configured.
func (s *Service) HasWebhookStore() bool {
	return s.webhookStore != nil
}

// CreateWebhookResult contains the result of creating a webhook.
type CreateWebhookResult struct {
	Webhook   *auth.Webhook
	FullToken string // Only returned once at creation
}

// CreateWebhook creates a new webhook for a DAG.
func (s *Service) CreateWebhook(ctx context.Context, dagName, creatorID string) (*CreateWebhookResult, error) {
	if s.webhookStore == nil {
		return nil, ErrWebhookNotConfigured
	}

	if dagName == "" {
		return nil, auth.ErrInvalidWebhookDAGName
	}

	if creatorID == "" {
		return nil, ErrInvalidCreatorID
	}

	// Generate webhook token
	tokenParts, err := generateWebhookToken(s.config.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to generate webhook token: %w", err)
	}

	webhook, err := auth.NewWebhook(dagName, tokenParts.tokenHash, tokenParts.tokenPrefix, creatorID)
	if err != nil {
		return nil, err
	}

	if err := s.webhookStore.Create(ctx, webhook); err != nil {
		return nil, err
	}

	return &CreateWebhookResult{
		Webhook:   webhook,
		FullToken: tokenParts.fullToken,
	}, nil
}

// webhookTokenParts holds the components of a generated webhook token.
type webhookTokenParts struct {
	fullToken   string
	tokenPrefix string
	tokenHash   string
}

// generateWebhookToken generates a new webhook token with prefix, hash, and prefix for display.
func generateWebhookToken(bcryptCost int) (*webhookTokenParts, error) {
	randomBytes := make([]byte, webhookTokenRandomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	encoded := stringutil.Base58Encode(randomBytes)
	fullToken := webhookTokenPrefix + encoded

	// Store first N characters as prefix for identification
	var tokenPrefix string
	if len(fullToken) >= webhookTokenPrefixLength {
		tokenPrefix = fullToken[:webhookTokenPrefixLength]
	} else {
		tokenPrefix = fullToken
	}

	// Hash the full token
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(fullToken), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash webhook token: %w", err)
	}

	return &webhookTokenParts{
		fullToken:   fullToken,
		tokenPrefix: tokenPrefix,
		tokenHash:   string(hashBytes),
	}, nil
}

// GetWebhookByDAGName retrieves the webhook for a specific DAG.
func (s *Service) GetWebhookByDAGName(ctx context.Context, dagName string) (*auth.Webhook, error) {
	if s.webhookStore == nil {
		return nil, ErrWebhookNotConfigured
	}
	return s.webhookStore.GetByDAGName(ctx, dagName)
}

// ListWebhooks returns all webhooks.
func (s *Service) ListWebhooks(ctx context.Context) ([]*auth.Webhook, error) {
	if s.webhookStore == nil {
		return nil, ErrWebhookNotConfigured
	}
	return s.webhookStore.List(ctx)
}

// DeleteWebhook deletes a webhook by DAG name.
func (s *Service) DeleteWebhook(ctx context.Context, dagName string) error {
	if s.webhookStore == nil {
		return ErrWebhookNotConfigured
	}
	return s.webhookStore.DeleteByDAGName(ctx, dagName)
}

// RegenerateWebhookToken generates a new token for an existing webhook.
// The old token becomes invalid immediately.
func (s *Service) RegenerateWebhookToken(ctx context.Context, dagName string) (*CreateWebhookResult, error) {
	if s.webhookStore == nil {
		return nil, ErrWebhookNotConfigured
	}

	webhook, err := s.webhookStore.GetByDAGName(ctx, dagName)
	if err != nil {
		return nil, err
	}

	// Generate new token
	tokenParts, err := generateWebhookToken(s.config.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to generate webhook token: %w", err)
	}

	// Update webhook with new token
	webhook.TokenHash = tokenParts.tokenHash
	webhook.TokenPrefix = tokenParts.tokenPrefix
	webhook.UpdatedAt = time.Now().UTC()

	if err := s.webhookStore.Update(ctx, webhook); err != nil {
		return nil, err
	}

	return &CreateWebhookResult{
		Webhook:   webhook,
		FullToken: tokenParts.fullToken,
	}, nil
}

// ToggleWebhook enables or disables a webhook without changing the token.
func (s *Service) ToggleWebhook(ctx context.Context, dagName string, enabled bool) (*auth.Webhook, error) {
	if s.webhookStore == nil {
		return nil, ErrWebhookNotConfigured
	}

	webhook, err := s.webhookStore.GetByDAGName(ctx, dagName)
	if err != nil {
		return nil, err
	}

	webhook.Enabled = enabled
	webhook.UpdatedAt = time.Now().UTC()

	if err := s.webhookStore.Update(ctx, webhook); err != nil {
		return nil, err
	}

	return webhook, nil
}

// ValidateWebhookToken validates a webhook token for a specific DAG.
// Returns the webhook if valid and enabled.
func (s *Service) ValidateWebhookToken(ctx context.Context, dagName, token string) (*auth.Webhook, error) {
	if s.webhookStore == nil {
		return nil, ErrWebhookNotConfigured
	}

	// Check for correct prefix
	if !strings.HasPrefix(token, webhookTokenPrefix) {
		return nil, ErrInvalidWebhookToken
	}

	webhook, err := s.webhookStore.GetByDAGName(ctx, dagName)
	if err != nil {
		if errors.Is(err, auth.ErrWebhookNotFound) {
			return nil, ErrInvalidWebhookToken
		}
		return nil, err
	}

	// Validate token hash
	if err := bcrypt.CompareHashAndPassword([]byte(webhook.TokenHash), []byte(token)); err != nil {
		return nil, ErrInvalidWebhookToken
	}

	// Check if webhook is enabled
	if !webhook.Enabled {
		return nil, ErrWebhookDisabled
	}

	// Update last used timestamp
	if err := s.webhookStore.UpdateLastUsed(ctx, webhook.ID); err != nil {
		slog.Error("failed to update webhook last used timestamp", "webhookID", webhook.ID, "error", err)
	}

	return webhook, nil
}
