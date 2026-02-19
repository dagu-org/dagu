---
id: "018"
title: "Authentication by Default"
status: draft
---

# RFC 018: Authentication by Default

## Summary

Dagu v2 enables authentication by default. The auth mode changes from `none` to `builtin`, meaning every new installation starts with authentication enabled. To preserve a frictionless first-run experience, the server auto-generates its JWT signing secret and presents a browser-based setup page for creating the initial admin account — no `config.yaml` editing required.

This RFC also removes the standalone `auth.mode=oidc` in favor of OIDC as an addon to builtin auth. Standalone OIDC provided no user persistence, no role mapping, and hardcoded every authenticated user as admin — builtin+OIDC is a strict superset with proper user management, role mapping, and access control.

Additionally, this RFC promotes basic auth from a side-channel overlay (`auth.basic.enabled` + credentials) to a proper auth mode (`auth.mode: basic`), eliminating inconsistencies in how basic auth was enabled across different code paths.

Finally, this RFC introduces a pluggable auth store interface that unifies the persistence of users, API keys, and webhooks behind a single abstraction, and a type-safe token secret provider system with compile-time checks and auto-generation. The v2 implementation remains file-based, but the interfaces are designed so that database backends can be added later without changing the auth service layer.

---

## Motivation

### Security by Default

Running without authentication is a reasonable development convenience but a poor production default. Users who deploy Dagu and forget (or don't know) to enable auth expose their workflow engine — including DAG definitions, execution controls, environment variables, and terminal access — to anyone who can reach the server.

Changing the default to `builtin` ensures that security is opt-out rather than opt-in.

### First-Run Friction

Today, enabling builtin auth requires three manual steps before the server will start:

1. Set `auth.mode: builtin` in `config.yaml`
2. Set `auth.builtin.token.secret` to a random string (or set `DAGU_AUTH_TOKEN_SECRET`)
3. Optionally pre-configure admin credentials (otherwise a random password is printed to stdout and easily lost)

If any step is missed the server either refuses to start (missing secret) or starts with credentials the user never saw. This friction discourages adoption of authentication.

### OIDC Mode Consolidation

Dagu v1 offers two ways to use OIDC: standalone (`auth.mode=oidc`) and as an addon to builtin auth (`auth.mode=builtin` with OIDC configured). The standalone mode has significant limitations:

- **Hardcoded admin role** — every OIDC-authenticated user receives `RoleAdmin` regardless of their identity or group membership (`oidc.go:285`). There is no role mapping, no whitelist, and no domain restriction.
- **No user persistence** — users are ephemeral, created on-the-fly per request. There is no user store, so no user management, no disabling accounts, no audit trail of who logged in.
- **No API keys or webhooks** — these stores are not initialized in standalone OIDC mode, so programmatic access and webhook integrations are unavailable.
- **Cookie-based only** — authentication uses raw OIDC tokens in cookies rather than JWTs, limiting integration options.
- **Terminal disabled** — the web terminal requires an auth service, which is not initialized in standalone mode.

In contrast, builtin+OIDC provides full user persistence, role mapping (jq expressions, group mappings, default roles), whitelist/domain enforcement, JWT tokens, API keys, webhooks, and terminal access. It is a strict superset.

Maintaining two OIDC code paths doubles the surface area for security issues and confuses users who must understand the difference. Removing standalone OIDC simplifies the auth model to three clear modes: `none`, `basic`, and `builtin`.

### Basic Auth Inconsistencies

Basic auth in Dagu v1 is not an auth mode — it is an overlay configured via `auth.basic.enabled`, `auth.basic.username`, and `auth.basic.password` that sits alongside the mode system. This design has several problems:

- **Inconsistent enable conditions** — the API route handler (`buildAuthOptions` in `api.go`) enables basic auth when `Username != "" && Password != ""`, ignoring the `Enabled` boolean entirely. The agent route handler (`buildAgentAuthOptions` in `server.go`) checks `Basic.Enabled` instead. This means basic auth can be enabled for API routes but not agent routes (or vice versa) depending on which fields are set.
- **Warning contradicts behavior** — when `auth.mode=builtin`, the config loader warns that basic auth is "ignored" but the middleware still wires it as a working fallback. Users are told their config doesn't apply when it actually does.
- **Silently ignored in none mode** — when `auth.mode=none`, basic auth configuration is silently skipped even if explicitly enabled. The early return bypasses all auth setup.
- **No validation** — there is no check for empty username or password. Setting `auth.basic.enabled: true` with no credentials produces no error.
- **Hardcoded admin role** — all basic-auth users receive `RoleAdmin` with no RBAC differentiation.

Promoting basic auth to a proper mode (`auth.mode: basic`) eliminates these issues by giving it a single, unambiguous code path with proper validation.

### Scattered Auth Storage

Auth-related data is spread across three independent file-based stores (`fileuser`, `fileapikey`, `filewebhook`), each with its own initialization path, caching strategy, and error handling. The user store notably lacks the caching that API key and webhook stores have. There is no unified interface — making it difficult to swap the file backend for a database without touching every consumer.

---

## Design

### 1. Default Auth Mode

The default value of `auth.mode` changes from `""` (treated as `none`) to `builtin`.

| Mode | Behavior |
|------|----------|
| `builtin` | **(default)** JWT-based auth with RBAC, user management, API keys, webhooks. Optional OIDC addon via `auth.oidc` config. |
| `basic` | HTTP Basic Auth (simple username/password, all users get admin role) |
| `none` | No authentication — explicit opt-out required |

The standalone `auth.mode=oidc` is removed in v2. Users who previously used standalone OIDC must switch to `auth.mode=builtin` with OIDC configured — this provides the same SSO login flow plus user persistence, role mapping, and access control.

Users who want no auth must explicitly set `auth.mode: none` or `DAGU_AUTH_MODE=none`.

When `auth.mode=builtin` **(default)**:

- Auth stores are initialized (user store, API key store, webhook store)
- JWT signing secret is resolved via the `TokenSecretProvider` chain — auto-generated and persisted if not configured (Section 2)
- If no users exist → setup page mode (Section 3)
- RBAC enforced — users have roles (`admin`, `manager`, `developer`, `operator`, `viewer`)
- Optional OIDC addon via `auth.oidc` config for SSO login
- API keys and webhooks available for programmatic access

When `auth.mode=none`:

- No auth stores are initialized (no user store, no API key store, no webhook store)
- No JWT secret is generated or loaded
- No setup page is shown
- All endpoints are accessible without credentials
- The server logs a startup warning: `"Authentication is disabled. All endpoints are publicly accessible."`

This is a conscious opt-out. No auth infrastructure runs — the server behaves as Dagu v1 did by default.

When `auth.mode=basic`:

- Requires `auth.basic.username` and `auth.basic.password` to be set (validation error if missing)
- No auth stores are initialized (no user store, no API key store, no webhook store)
- No JWT secret is generated or loaded
- No setup page is shown
- All authenticated users receive `RoleAdmin` — there is no RBAC
- HTTP Basic Auth is enforced on all non-public endpoints (both API and agent routes)
- Intended for simple deployments, CI environments, or quick demos where full user management is unnecessary

The `auth.basic.enabled` boolean field is removed — mode selection replaces it. Setting `auth.basic` credentials when mode is not `basic` is a **validation error**, not a warning. This eliminates the v1 ambiguity where basic auth could be half-enabled alongside other modes.

### 2. Token Secret: Type-Safe Resolution

The JWT signing secret is promoted from a plain `string` to an opaque type with compile-time safety, and its resolution is abstracted behind a provider interface that supports auto-generation.

#### Problem with the Current Design

Today, the secret is a `string` at every layer — config, auth service, JWT signing. This allows accidental logging (`fmt.Printf("%s", secret)`), accidental JSON serialization, and confusion with any other string. The secret has no abstraction over its source, and the server hard-errors if the secret is empty, preventing auto-generation.

#### `TokenSecret` Opaque Type

**Location:** `internal/auth/token_secret.go`

```go
// TokenSecret is an opaque handle to JWT signing key material.
// The zero value is invalid, forcing callers through constructors.
type TokenSecret struct {
    key []byte // unexported: prevents direct access
}

func NewTokenSecret(key []byte) (TokenSecret, error)
func NewTokenSecretFromString(s string) (TokenSecret, error)

func (ts TokenSecret) SigningKey() []byte   // ONLY way to access key material
func (ts TokenSecret) IsValid() bool        // false for zero value
func (ts TokenSecret) String() string       // → "[REDACTED]"
func (ts TokenSecret) GoString() string     // → "auth.TokenSecret{[REDACTED]}"
func (ts TokenSecret) MarshalJSON() ([]byte, error)  // → "[REDACTED]"
func (ts TokenSecret) MarshalText() ([]byte, error)  // → "[REDACTED]"
```

**Design rationale:**

- **struct with unexported field** (not `type TokenSecret string`): prevents `TokenSecret("oops")` and `string(secret)`. The zero value is invalid, forcing explicit construction via `NewTokenSecret` or `NewTokenSecretFromString`.
- **`[]byte` internally** (not `string`): the JWT library (`golang-jwt/jwt/v5`) expects `[]byte` for HMAC signing. Storing as `[]byte` avoids a conversion on every sign/verify.
- **Defensive copy in constructors**: the input is copied to prevent the caller from mutating the underlying key after construction.
- **`SigningKey()` is intentionally named**: signals purpose ("this is for signing") rather than being a generic getter.

#### `TokenSecretProvider` Interface

**Location:** `internal/auth/token_secret.go` (same file as the type)

```go
// TokenSecretProvider resolves the JWT signing secret from a configured source.
type TokenSecretProvider interface {
    Resolve(ctx context.Context) (TokenSecret, error)
}
```

**Design rationale:**

- **Returns `TokenSecret`** (not `string` or `[]byte`): compile-time type safety — a mismatched type is a compile error, not a runtime surprise.
- **Single method**: follows Go convention for small, composable interfaces.
- **"Resolve" not "Get"**: signals that the provider may perform side effects (file I/O, auto-generation) beyond a simple read.
- **`context.Context`**: enables future providers that call external services (Vault, cloud KMS) to respect deadlines.

#### Provider Implementations

**Location:** `internal/auth/tokensecret/` (new sub-package)

This follows the existing pattern: `internal/auth` defines domain types + interfaces, sub-packages (`fileuser`, `fileapikey`, `filewebhook`) implement them.

| Provider | Source | Behavior |
|----------|--------|----------|
| `StaticProvider` | Config/env string | Wraps a known value. No I/O, no side effects. |
| `FileProvider` | `{DataDir}/auth/token_secret` | Reads existing file. If missing, generates 32 bytes via `crypto/rand`, base64url-encodes (43 chars), writes with `0600` perms / `0700` dir. |
| `ChainProvider` | Ordered list of providers | Tries each in order. `ErrInvalidTokenSecret` = skip to next. Other errors = fatal (e.g., file permission denied is not silently skipped). |

#### Resolution Priority

The chain is constructed at startup in `buildTokenSecretProvider()`:

```
1. StaticProvider  — if config/env has a non-empty value
2. FileProvider    — reads {DataDir}/auth/token_secret, or auto-generates
```

The chain always contains at least `FileProvider`, so resolution always succeeds. The hard startup error for missing secret is removed — the server can always start in `builtin` mode.

**Properties:**

- **Persistent** — auto-generated secrets survive server restarts (persisted to file)
- **Secure** — file is `0600`, readable only by the server process owner; directory is `0700`
- **Overridable** — config/env value takes precedence over file
- **Rotatable** — delete the file and restart; `FileProvider` auto-generates a new one; all existing JWTs invalidate

#### Data Flow

The end-to-end flow from configuration to JWT signing, showing where each type boundary is crossed:

```
YAML / env var
    │
    ▼
Viper unmarshal → Definition.Auth.Builtin.Token.Secret (string)
    │
    ▼
loadBuiltinAuth() → Config...Token.Secret (string)  [unchanged — Viper needs string]
    │
    ▼
buildTokenSecretProvider()
    ├─ non-empty? → StaticProvider(string) → TokenSecret
    └─ FileProvider({DataDir}/auth/)
         ├─ file exists? → read → TokenSecret
         └─ file missing? → generate, persist, return → TokenSecret
    │
    ▼
ChainProvider.Resolve() → auth.TokenSecret (opaque)
    │
    ▼
authservice.Config{TokenSecret: auth.TokenSecret}
    │
    ▼
service.config.TokenSecret.SigningKey() → []byte  [at JWT boundary only]
    │
    ▼
jwt.SignedString(signingKey) / jwt.Parse(... signingKey)
```

**Key boundaries:**

- **Config layer remains `string`**: `TokenConfig.Secret` stays a `string` because Viper cannot unmarshal into opaque types. This is the input layer.
- **Single conversion point**: `buildTokenSecretProvider()` is where `string` → `auth.TokenSecret` happens. This is the only place in the codebase that touches raw secret strings.
- **Auth service uses opaque type**: `authservice.Config.TokenSecret` is `auth.TokenSecret` (not `string`). The compiler enforces this at every call site.
- **JWT library gets `[]byte`**: `SigningKey()` is called only inside `GenerateToken()` and `ValidateToken()` — the narrowest possible scope.

#### Generation

`FileProvider` generates 32 bytes from `crypto/rand`, base64url-encodes to a 43-character printable string (human-inspectable, can be copy-pasted for multi-instance deployments), and writes to `{DataDir}/auth/token_secret` with `0600` permissions in a `0700` directory.

#### Rotation

Delete the secret file and restart:

```bash
rm ~/.dagu/data/auth/token_secret
dagu restart
```

`FileProvider` detects the missing file and auto-generates a new secret. All existing JWT tokens become invalid. Users must re-authenticate. User accounts, API keys, and webhooks are unaffected.

#### Package Layout

```
internal/auth/
    token_secret.go          # TokenSecret type, TokenSecretProvider interface,
                             # ErrInvalidTokenSecret, IsNotAvailable helper

internal/auth/tokensecret/
    static.go                # StaticProvider — wraps config/env value
    file.go                  # FileProvider — file read/write + auto-generation
    chain.go                 # ChainProvider — tries providers in priority order
```

This follows the existing codebase pattern: `internal/auth` defines domain types and interfaces, sub-packages (`fileuser`, `fileapikey`, `filewebhook`) provide implementations.

### 3. Initial Setup Flow

When the server starts in `builtin` mode and the user store is empty (zero users), it enters **setup mode** instead of auto-creating an admin with a printed password.

#### Setup State Detection

Setup mode only applies when `auth.mode=builtin`. The server checks `userStore.Count() == 0` at startup (after auth store initialization). If true, it sets `setupRequired=true`, which is passed to the frontend via the existing template config injection — the same mechanism already used for `authMode`, `oidcEnabled`, and other config flags. No separate API endpoint is needed for detecting setup state.

#### Setup API Endpoint

```
POST /api/v1/auth/setup
```

- **Public** — no authentication required (added to `PublicPaths`)
- **One-time** — returns `403 Forbidden` if any users already exist
- **Atomic** — checks count and creates user within the same service call to prevent races

**Request:**

```json
{
  "username": "string (required, min 1)",
  "password": "string (required, min 8)"
}
```

**Response (200):**

```json
{
  "token": "eyJhbGci...",
  "expiresAt": "2025-07-20T12:00:00Z",
  "user": {
    "id": "uuid",
    "username": "admin",
    "role": "admin"
  }
}
```

The response mirrors the login response — the user is immediately authenticated after setup.

#### Frontend Setup Page

A new `/setup` route presents a clean welcome screen:

- Application name/logo
- "Create your admin account" heading
- Username field (default placeholder: `admin`)
- Password field (with strength indicator)
- Password confirmation field
- "Create Account" button

**Redirect logic:**

- If `setupRequired` is true and user visits any page → redirect to `/setup`
- If `setupRequired` is false and user visits `/setup` → redirect to `/login`
- After successful setup → store token, redirect to dashboard

### 4. Auth Store Interface

A new unified interface abstracts all auth-related persistence. The current three stores (`fileuser`, `fileapikey`, `filewebhook`) are consolidated behind a single `AuthStore` interface that the auth service depends on.

#### Interface Design

```go
// AuthStore is the unified interface for all auth-related persistence.
// Implementations must be safe for concurrent use.
//
// Note: Token secret resolution is handled by TokenSecretProvider (Section 2),
// not by AuthStore. The secret is resolved once at startup, not on every
// request, so it does not need the same persistence abstraction as users,
// API keys, and webhooks.
type AuthStore interface {
    UserStore
    APIKeyStore
    WebhookStore
}
```

`UserStore`, `APIKeyStore`, and `WebhookStore` retain their existing interfaces (already defined in `internal/auth/store.go`).

**Why no `SecretStore` sub-interface?**

- **YAGNI** — the only secret the system manages is the JWT signing key. A generic `GetSecret(name string) → []byte` interface is speculative generalization.
- **Stringly-typed** — the `name` parameter has no compile-time safety. A typo like `"token_secert"` is a runtime error, not a compile error.
- **Wrong abstraction level** — `SecretStore` mixes "where the secret comes from" with "how the secret is stored". `TokenSecretProvider` (Section 2) cleanly separates these: each implementation handles one source, and `ChainProvider` handles priority.
- **Future extensibility** — if other secrets are needed later (e.g., encryption-at-rest keys), a new typed provider (`EncryptionKeyProvider`) is trivial to add and provides the same compile-time safety.

#### File-Based Implementation

The file-based implementation composes the existing stores:

```go
type FileAuthStore struct {
    users    *fileuser.Store
    apiKeys  *fileapikey.Store
    webhooks *filewebhook.Store
}
```

Token secret file management is internal to `FileProvider` (Section 2), not the auth store. The auth store is concerned with per-request data (users, API keys, webhooks); the token secret is a startup-time concern resolved once.

#### User Store Caching

The user store gains optional LRU+TTL caching, aligned with the API key and webhook stores:

```go
userCache := fileutil.NewCache[*auth.User](
    "user",
    cacheLimits.User.Limit,  // e.g., Normal: 500
    cacheLimits.User.TTL,    // e.g., 15 minutes
)
userStore, err := fileuser.New(cfg.Paths.UsersDir,
    fileuser.WithFileCache(userCache),
)
```

This avoids disk reads on every JWT validation (which calls `GetUserFromToken` → `store.GetByID`). The cache uses the same staleness-detection pattern as the existing API key cache: checking file size and modification time before returning cached data.

#### Initialization Simplification

Currently, the server initializes each store independently with separate error handling, cache setup, and wiring. With the unified interface, initialization becomes:

```go
authStore, err := fileauth.New(fileauth.Config{
    UsersDir:    cfg.Paths.UsersDir,
    APIKeysDir:  cfg.Paths.APIKeysDir,
    WebhooksDir: cfg.Paths.WebhooksDir,
    CacheLimits: cfg.Cache.Limits(),
})

// Resolve token secret through the provider chain (Section 2)
secretProvider := buildTokenSecretProvider(cfg)
tokenSecret, err := secretProvider.Resolve(ctx)

authSvc := authservice.New(authStore, authservice.Config{
    TokenSecret: tokenSecret,  // auth.TokenSecret (opaque type), not string
    TokenTTL:    cfg.Server.Auth.Builtin.Token.TTL,
})
```

The auth store handles internal cache setup, eviction scheduling, and telemetry registration. The token secret is resolved separately by the provider chain and injected into the auth service config as an opaque `auth.TokenSecret` value.

---

## User Experience Flows

### First-Time Installation

```
1. Install Dagu, run `dagu start-all`
2. Server starts → auth.mode defaults to builtin
3. Auto-generates JWT secret → persists to {DataDir}/auth/token_secret
4. Detects 0 users → sets setupRequired=true
5. User opens http://localhost:8080
6. Frontend redirects to /setup
7. "Welcome to Dagu — Create your admin account"
8. User enters username + password
9. POST /api/v1/auth/setup → creates admin → returns JWT
10. Frontend stores token → redirects to dashboard
```

### Explicit No-Auth (Development)

```yaml
# config.yaml
auth:
  mode: none
```

```
1. Server starts → mode=none
2. No auth initialization, no setup page
3. All endpoints accessible without credentials
```

---

## Relationship to Existing Features

| Feature | How this RFC relates |
|---------|---------------------|
| **Builtin auth** | Becomes the default mode; initialization simplified |
| **OIDC auth** | Standalone `auth.mode: oidc` removed; OIDC available exclusively as addon to builtin mode via `auth.oidc` config |
| **Basic auth** | Promoted from overlay (`auth.basic.enabled`) to proper mode (`auth.mode: basic`); `Enabled` flag removed; setting basic credentials under any other mode is a validation error |
| **API keys** | Store moves behind `AuthStore` interface; behavior unchanged |
| **Webhooks** | Store moves behind `AuthStore` interface; behavior unchanged |
| **Audit logging** | Setup actions logged as audit events |
| **Agent/Terminal** | Already requires builtin auth; now available by default |
| **Remote nodes** | Auth settings remain per-node; unaffected |

---

## Examples

### Minimal Config (Auth Enabled by Default)

```yaml
# config.yaml — no auth section needed
port: 8080
```

Auth is enabled automatically. Secret auto-generated. Setup page on first visit.

### Explicit No-Auth for Development

```yaml
auth:
  mode: none
```

### Basic Auth (Simple Password Protection)

```yaml
auth:
  mode: basic
  basic:
    username: "admin"
    password: "s3cret"
```

Single-user auth with no JWT, no user management, no RBAC. All requests require HTTP Basic Auth credentials.

### Custom Token Secret (for Multi-Instance Deployments)

```yaml
auth:
  builtin:
    token:
      secret: "my-shared-secret-across-instances"
      ttl: 12h
```

All instances share the same secret → JWTs are valid across instances.

### Builtin Auth with OIDC (SSO Login)

```yaml
auth:
  mode: builtin  # or omit — builtin is the default
  oidc:
    client_id: "dagu-app"
    client_secret: "secret"
    client_url: "http://localhost:8080"
    issuer: "https://accounts.google.com"
    scopes: [openid, profile, email]
    auto_signup: true
    role_mapping:
      default_role: viewer
      groups_claim: groups
      group_mappings:
        platform-admins: admin
        developers: developer
```

OIDC users are created/updated on first login with role mapping applied. The login page shows both local credentials and a "Login with SSO" button.

---

## Risks

1. **Breaking change for existing users** — Users upgrading to v2 with no auth config will suddenly require authentication. Mitigated by: v2 is an explicit major version bump; release notes will clearly document the change.

2. **Setup page exposure window** — Between server start and admin account creation, the setup endpoint is publicly accessible. Mitigated by: the endpoint is one-time-use (disabled after first user creation) and only accepts username + password (no privilege escalation possible).

3. **Auto-generated secret on ephemeral filesystems** — Containers without persistent volumes will generate a new secret on every restart, invalidating all JWTs. Mitigated by: documented requirement for persistent storage in container deployments; alternatively, set `DAGU_AUTH_TOKEN_SECRET` explicitly.

4. **User store cache coherence** — Adding caching to the user store introduces potential for stale reads (e.g., disabled user still cached as enabled). Mitigated by: same staleness-detection pattern already proven in API key and webhook stores; cache invalidation on write operations.

5. **Standalone OIDC removal** — Existing users with `auth.mode=oidc` must migrate to `auth.mode=builtin` with OIDC configured. Mitigated by: v2 is a major version bump; migration is straightforward (add OIDC fields under builtin config); the new mode is strictly more capable.

6. **Basic auth config migration** — Users with `auth.basic.enabled: true` must change to `auth.mode: basic`. Users who relied on basic auth as a fallback alongside builtin mode must choose one mode. Mitigated by: v2 is a major version bump; the server emits a clear validation error pointing users to the correct config structure.

---

## Code Cleanup

### Standalone OIDC Removal

Removing standalone `auth.mode=oidc` allows deletion of the following code paths:

- `OIDCMiddleware()` in `internal/service/frontend/auth/middleware.go` — standalone OIDC middleware for UI routes
- `checkOIDCAuth()` in `internal/service/frontend/auth/oidc.go` — standalone OIDC UI auth handler
- `checkOIDCToken()` in `internal/service/frontend/auth/oidc.go` — standalone OIDC API auth handler
- `verifyAndExtractUser()` in `internal/service/frontend/auth/oidc.go` — ephemeral user creation with hardcoded `RoleAdmin`
- `AuthModeOIDC` constant in `internal/cmn/config/config.go` — no longer a valid auth mode
- `autoDetectAuthMode()` in `internal/cmn/config/loader.go` — OIDC auto-detection logic
- Standalone OIDC routing branch in `setupRoutes()` in `internal/service/frontend/server.go`

The builtin+OIDC code paths (`BuiltinOIDCCallbackHandler`, `setupOIDCRoutes`, OIDC provisioning service) remain unchanged.

### Basic Auth Overlay Removal

Promoting basic auth to a proper mode requires the following changes:

- Add `AuthModeBasic` constant to `internal/cmn/config/config.go` — new valid mode alongside `none` and `builtin`
- Remove `Auth.Basic.Enabled` field from `AuthBasic` struct in `internal/cmn/config/config.go` — mode selection replaces the boolean
- Add `validateBasicAuth()` to `internal/cmn/config/config.go` — require non-empty username and password when `auth.mode=basic`; reject basic credentials under any other mode
- Replace username/password presence check in `buildAuthOptions()` (`internal/service/frontend/api/v1/api.go`) with `Mode == AuthModeBasic`
- Replace `Basic.Enabled` check in `buildAgentAuthOptions()` (`internal/service/frontend/server.go`) with `Mode == AuthModeBasic`
- Remove `warnBasicAuthWithBuiltin()` in `internal/cmn/config/loader.go` — the warning is replaced by a validation error
- Remove `loadBasicAuth()` loading of the `Enabled` field in `internal/cmn/config/loader.go`
- Update config schema enum in `internal/cmn/schema/config.schema.json` to `["none", "basic", "builtin"]`
- Remove `enabled` field from `AuthBasicDef` in `internal/cmn/config/definition.go`

### Token Secret Type Safety

Promoting the token secret from a plain `string` to the opaque `TokenSecret` type requires the following changes:

- Add `TokenSecret` type, `TokenSecretProvider` interface, constructors, and helpers to `internal/auth/token_secret.go`
- Add `StaticProvider`, `FileProvider`, and `ChainProvider` implementations in `internal/auth/tokensecret/`
- Change `authservice.Config.TokenSecret` from `string` to `auth.TokenSecret` in `internal/service/auth/service.go` — the compiler catches all call sites
- Change `[]byte(s.config.TokenSecret)` to `s.config.TokenSecret.SigningKey()` in `internal/service/auth/service.go` (GenerateToken and ValidateToken)
- Change `s.config.TokenSecret == ""` to `!s.config.TokenSecret.IsValid()` in `internal/service/auth/service.go`
- Remove `Token.Secret == ""` hard error from `validateBuiltinAuth()` in `internal/cmn/config/config.go` — auto-generation makes empty secret a valid state
- Remove redundant `Token.Secret == ""` guard from `initBuiltinAuthService()` in `internal/service/frontend/server.go`
- Add `buildTokenSecretProvider()` factory function to `internal/service/frontend/server.go` — constructs the provider chain and resolves the secret at startup

---

## Design Decisions

| Decision | Chosen | Alternative | Rationale |
|----------|--------|-------------|-----------|
| Secret representation | struct with unexported `[]byte` | `type TokenSecret string` | Struct prevents `TokenSecret("oops")` and `string(secret)` — zero value is invalid, forces explicit construction |
| Internal storage | `[]byte` | `string` | JWT library expects `[]byte`; storing as `[]byte` avoids conversion per sign/verify |
| Provider interface | Typed `TokenSecretProvider` → `TokenSecret` | Generic `SecretStore` → `(name, []byte)` | Compile-time safety; no magic strings; single-purpose |
| Provider location | `internal/auth/tokensecret/` sub-package | Inline in `internal/auth/` | Keeps domain model clean; mirrors existing `fileuser`, `fileapikey` pattern |
| SecretStore in AuthStore | Removed | Kept as sub-interface | YAGNI; only one secret exists; typed provider is safer |
| Config layer type | Remains `string` | Change to `TokenSecret` | Viper cannot unmarshal into opaque types; conversion at boundary |
| Empty secret validation | Removed from `validateBuiltinAuth()` | Keep as warning | Auto-generation makes empty a valid state; warning would confuse |
| Basic auth representation | Proper `AuthModeBasic` constant | Overlay with `Enabled` boolean | Mode-based eliminates inconsistent enable checks across code paths |

---

## Out of Scope

- Database-backed auth store implementation (this RFC defines the interface only; file-based is the v2 implementation)
- Multi-factor authentication (MFA/2FA)
- Password complexity rules beyond minimum length
- Account lockout after failed attempts
- Session revocation / token blocklist
- Email-based password recovery
- Self-service user registration (only admin can create additional users after initial setup)
