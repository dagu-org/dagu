# User Management and RBAC Requirement Specification

## 1. Overview

This document outlines the requirements and design for implementing User Management and Role-Based Access Control (RBAC) in Dagu. The goal is to add a `builtin` authentication mode with multi-user support and granular permissions.

### Authentication Modes

Dagu supports three mutually exclusive authentication modes:

| Mode | Description |
|------|-------------|
| `none` | No authentication (default, for local/dev use) |
| `builtin` | Internal user management with RBAC (new) |
| `oidc` | External OIDC provider with whitelist (existing, unchanged) |

## 2. Goals & Principles

* **Robustness**: Reliable user data management and authentication flows.
* **Security**: Industry-standard practices for password hashing (bcrypt) and JWT tokens.
* **Scalability**: Storage interface supports multiple backends (file-based initially, extensible to SQL/NoSQL).
* **Backward Compatibility**: Existing `oidc` mode continues to work unchanged.

## 3. Configuration

```yaml
auth:
  mode: "none"  # "none" | "builtin" | "oidc"

  # When mode: "builtin"
  builtin:
    default_admin:
      username: "admin"
      password: "${DAGU_ADMIN_PASSWORD}"  # Generated and printed to stdout if not set
    token:
      secret: "${AUTH_TOKEN_SECRET}"      # Required for JWT signing
      ttl: "24h"                          # Token time-to-live

  # When mode: "oidc" (existing, unchanged)
  oidc:
    clientId: "..."
    clientSecret: "..."
    clientUrl: "..."
    issuer: "..."
    scopes: ["openid", "profile", "email"]
    whitelist: ["user@example.com"]
```

### 3.1. Configuration Struct

```go
type AuthConfig struct {
    Mode    string        `yaml:"mode"`    // "none", "builtin", "oidc"
    Builtin BuiltinConfig `yaml:"builtin"`
    OIDC    AuthOIDC      `yaml:"oidc"`    // Existing struct, unchanged
}

type BuiltinConfig struct {
    DefaultAdmin DefaultAdminConfig `yaml:"default_admin"`
    Token        TokenConfig        `yaml:"token"`
}

type DefaultAdminConfig struct {
    Username string `yaml:"username"`
    Password string `yaml:"password"`
}

type TokenConfig struct {
    Secret string        `yaml:"secret"`
    TTL    time.Duration `yaml:"ttl"`
}
```

## 4. Data Model

### 4.1. User Entity

```go
// internal/core/auth/user.go

type User struct {
    ID           string    `json:"id"`         // UUID
    Username     string    `json:"username"`   // Unique login name
    PasswordHash string    `json:"-"`          // bcrypt hash
    Role         Role      `json:"role"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}
```

### 4.2. Role Type

```go
// internal/core/auth/role.go

type Role string

const (
    RoleAdmin  Role = "admin"
    RoleEditor Role = "editor"
    RoleViewer Role = "viewer"
)

func (r Role) Valid() bool {
    switch r {
    case RoleAdmin, RoleEditor, RoleViewer:
        return true
    }
    return false
}

func (r Role) CanWrite() bool {
    return r == RoleAdmin || r == RoleEditor
}

func (r Role) IsAdmin() bool {
    return r == RoleAdmin
}
```

### 4.3. Role Permissions

| Action | Admin | Editor | Viewer |
|--------|:-----:|:------:|:------:|
| View DAGs | ✓ | ✓ | ✓ |
| Create/Edit/Delete DAGs | ✓ | ✓ | ✗ |
| Start/Stop/Retry DAG Runs | ✓ | ✓ | ✗ |
| Manage Users | ✓ | ✗ | ✗ |

*Future extensibility*: Allow defining custom roles with specific permission sets (e.g., `dag:read`, `dag:write`, `run:start`).

## 5. Architecture

### 5.1. Core Interfaces (`internal/core/auth`)

```go
// internal/core/auth/store.go

type UserStore interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id string) (*User, error)
    GetByUsername(ctx context.Context, username string) (*User, error)
    List(ctx context.Context) ([]*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
    Count(ctx context.Context) (int64, error)
}
```

### 5.2. Persistence Layer (`internal/persistence/fileuser`)

* **Storage Format**: JSON
* **Directory Structure**: `{data_dir}/users/{user_id}.json`
* **Concurrency**: File-level locking (consistent with existing patterns)

### 5.3. Auth Service (`internal/service/auth`)

```go
type Service struct {
    userStore UserStore
    config    BuiltinConfig
}

// Authentication
func (s *Service) Authenticate(ctx context.Context, username, password string) (*User, error)
func (s *Service) GenerateToken(user *User) (string, error)
func (s *Service) ValidateToken(token string) (*User, error)

// User Management
func (s *Service) CreateUser(ctx context.Context, input CreateUserInput) (*User, error)
func (s *Service) GetUser(ctx context.Context, id string) (*User, error)
func (s *Service) ListUsers(ctx context.Context) ([]*User, error)
func (s *Service) UpdateUser(ctx context.Context, id string, input UpdateUserInput) (*User, error)
func (s *Service) DeleteUser(ctx context.Context, id string) error

// Password
func (s *Service) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error

// Initialization
func (s *Service) EnsureDefaultAdmin(ctx context.Context) error
```

### 5.4. Middleware (`internal/service/frontend/auth`)

```go
// RequireAuth validates JWT token and injects user into context.
func RequireAuth(authService *Service) func(http.Handler) http.Handler

// RequireRole checks if authenticated user has required role.
func RequireRole(roles ...Role) func(http.Handler) http.Handler
```

### 5.5. Context Helpers (`internal/core/auth/context.go`)

```go
func UserFromContext(ctx context.Context) (*User, bool)
func WithUser(ctx context.Context, user *User) context.Context
```

## 6. API Design

All endpoints use `/api/v2/` prefix. Only available when `mode: "builtin"`.

### 6.1. Authentication

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `POST` | `/api/v2/auth/login` | Login, returns JWT | Public |
| `POST` | `/api/v2/auth/logout` | Logout | Authenticated |
| `GET` | `/api/v2/auth/me` | Get current user | Authenticated |
| `PUT` | `/api/v2/auth/password` | Change own password | Authenticated |

### 6.2. User Management

| Method | Endpoint | Description | Auth |
|--------|----------|-------------|------|
| `GET` | `/api/v2/users` | List users | Admin |
| `POST` | `/api/v2/users` | Create user | Admin |
| `GET` | `/api/v2/users/{id}` | Get user | Admin |
| `PATCH` | `/api/v2/users/{id}` | Update user | Admin |
| `DELETE` | `/api/v2/users/{id}` | Delete user | Admin |

### 6.3. Request/Response Examples

**Login:**
```json
POST /api/v2/auth/login
{
    "username": "admin",
    "password": "password"
}

Response:
{
    "user": {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "username": "admin",
        "role": "admin",
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z"
    },
    "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Create User:**
```json
POST /api/v2/users
{
    "username": "john",
    "password": "securepassword",
    "role": "editor"
}
```

**Change Password:**
```json
PUT /api/v2/auth/password
{
    "old_password": "oldpass",
    "new_password": "newpass"
}
```

## 7. Error Response

```go
type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `auth.invalid_credentials` | 401 | Invalid username or password |
| `auth.token_invalid` | 401 | Invalid or expired token |
| `auth.unauthorized` | 401 | Authentication required |
| `auth.forbidden` | 403 | Insufficient permissions |
| `user.not_found` | 404 | User not found |
| `user.already_exists` | 409 | Username already taken |
| `validation.failed` | 400 | Request validation failed |

## 8. Security Considerations

* **Password Hashing**: bcrypt with cost factor 12
* **JWT Tokens**: HS256 signing, configurable TTL
* **Token Storage**: Client stores in memory or localStorage
* **Authorization Header**: `Authorization: Bearer <token>`

## 9. Implementation Phases

### Phase 1: Core & Persistence
1. Define `User`, `Role` types in `internal/core/auth/`
2. Define `UserStore` interface
3. Implement `fileuser` persistence
4. Implement `AuthService` with JWT

### Phase 2: API & Middleware
1. Add `RequireAuth` and `RequireRole` middleware
2. Implement auth endpoints (`/login`, `/logout`, `/me`, `/password`)
3. Implement user management endpoints
4. Create default admin on startup

### Phase 3: UI Integration
1. Add login page
2. Add user management page (Admin)
3. Hide/show UI elements based on role
