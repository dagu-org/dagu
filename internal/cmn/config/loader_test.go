package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLoad(t *testing.T, opts ...ConfigLoaderOption) *Config {
	t.Helper()
	cfg, err := NewConfigLoader(viper.New(), opts...).Load()
	require.NoError(t, err)
	return cfg
}

func testLoadWithError(t *testing.T, opts ...ConfigLoaderOption) error {
	t.Helper()
	_, err := NewConfigLoader(viper.New(), opts...).Load()
	return err
}

func TestLoad_Env(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("# minimal config"), 0600)
	require.NoError(t, err)

	testPaths := filepath.Join(tempDir, "test")

	testEnvs := map[string]string{
		"DAGU_LOG_FORMAT":   "json",
		"DAGU_BASE_PATH":    "/test/base",
		"DAGU_API_BASE_URL": "/test/api",
		"DAGU_TZ":           "Europe/Berlin",
		"DAGU_HOST":         "test.example.com",
		"DAGU_PORT":         "9876",
		"DAGU_DEBUG":        "true",
		"DAGU_HEADLESS":     "true",

		"DAGU_WORK_DIR":      filepath.Join(testPaths, "work"),
		"DAGU_DEFAULT_SHELL": "/bin/zsh",

		"DAGU_UI_MAX_DASHBOARD_PAGE_LIMIT": "250",
		"DAGU_UI_LOG_ENCODING_CHARSET":     "iso-8859-1",
		"DAGU_UI_NAVBAR_COLOR":             "#123456",
		"DAGU_UI_NAVBAR_TITLE":             "Test Dagu",

		"DAGU_AUTH_BASIC_USERNAME": "testuser",
		"DAGU_AUTH_BASIC_PASSWORD": "testpass",
		"DAGU_AUTH_TOKEN":          "test-token-123",

		"DAGU_CERT_FILE": filepath.Join(testPaths, "cert.pem"),
		"DAGU_KEY_FILE":  filepath.Join(testPaths, "key.pem"),

		"DAGU_DAGS_DIR":             filepath.Join(testPaths, "dags"),
		"DAGU_EXECUTABLE":           filepath.Join(testPaths, "bin", "dagu"),
		"DAGU_LOG_DIR":              filepath.Join(testPaths, "logs"),
		"DAGU_DATA_DIR":             filepath.Join(testPaths, "data"),
		"DAGU_SUSPEND_FLAGS_DIR":    filepath.Join(testPaths, "suspend"),
		"DAGU_ADMIN_LOG_DIR":        filepath.Join(testPaths, "admin"),
		"DAGU_BASE_CONFIG":          filepath.Join(testPaths, "base.yaml"),
		"DAGU_DAG_RUNS_DIR":         filepath.Join(testPaths, "runs"),
		"DAGU_PROC_DIR":             filepath.Join(testPaths, "proc"),
		"DAGU_QUEUE_DIR":            filepath.Join(testPaths, "queue"),
		"DAGU_SERVICE_REGISTRY_DIR": filepath.Join(testPaths, "service-registry"),

		"DAGU_LATEST_STATUS_TODAY": "true",

		"DAGU_QUEUE_ENABLED": "false",

		"DAGU_COORDINATOR_HOST":      "0.0.0.0",
		"DAGU_COORDINATOR_ADVERTISE": "dagu-coordinator",
		"DAGU_COORDINATOR_PORT":      "50099",

		"DAGU_WORKER_ID":              "test-worker-123",
		"DAGU_WORKER_MAX_ACTIVE_RUNS": "200",

		"DAGU_SCHEDULER_PORT":                      "9999",
		"DAGU_SCHEDULER_ZOMBIE_DETECTION_INTERVAL": "90s",

		"DAGU_AUTH_OIDC_CLIENT_ID":     "test-client-id",
		"DAGU_AUTH_OIDC_CLIENT_SECRET": "test-secret",
		"DAGU_AUTH_OIDC_ISSUER":        "https://auth.example.com",
		"DAGU_AUTH_OIDC_SCOPES":        "openid,profile,email",

		"DAGU_UI_DAGS_SORT_FIELD": "status",
		"DAGU_UI_DAGS_SORT_ORDER": "desc",

		"DAGU_TERMINAL_ENABLED": "true",

		"DAGU_AUDIT_ENABLED": "false",

		"DAGU_TUNNEL_ENABLED":                              "true",
		"DAGU_TUNNEL_TAILSCALE_AUTH_KEY":                   "tskey-test-123",
		"DAGU_TUNNEL_TAILSCALE_HOSTNAME":                   "test-dagu",
		"DAGU_TUNNEL_TAILSCALE_FUNNEL":                     "false",
		"DAGU_TUNNEL_TAILSCALE_HTTPS":                      "true",
		"DAGU_TUNNEL_ALLOW_TERMINAL":                       "true",
		"DAGU_TUNNEL_RATE_LIMITING_ENABLED":                "true",
		"DAGU_TUNNEL_RATE_LIMITING_LOGIN_ATTEMPTS":         "10",
		"DAGU_TUNNEL_RATE_LIMITING_WINDOW_SECONDS":         "600",
		"DAGU_TUNNEL_RATE_LIMITING_BLOCK_DURATION_SECONDS": "1800",
	}

	for key, val := range testEnvs {
		t.Setenv(key, val)
	}

	cfg := testLoad(t, WithConfigFile(configFile))

	berlinLoc, _ := time.LoadLocation("Europe/Berlin")
	_, berlinOffset := time.Now().In(berlinLoc).Zone()

	require.NotEmpty(t, cfg.Paths.ConfigFileUsed)
	cfg.Paths.ConfigFileUsed = ""

	expected := &Config{
		Core: Core{
			Debug:         true,
			LogFormat:     "json",
			TZ:            "Europe/Berlin",
			TzOffsetInSec: berlinOffset,
			Location:      berlinLoc,
			DefaultShell:  "/bin/zsh",
			SkipExamples:  false,
			Peer:          Peer{Insecure: true}, // Default is true
			BaseEnv:       cfg.Core.BaseEnv,     // Dynamic, copy from actual
		},
		Server: Server{
			Host:        "test.example.com",
			Port:        9876,
			BasePath:    "/test/base",
			APIBasePath: "/test/api",
			Headless:    true,
			Auth: Auth{
				Mode:  AuthModeOIDC, // Auto-detected from OIDC config
				Basic: AuthBasic{Username: "testuser", Password: "testpass"},
				Token: AuthToken{Value: "test-token-123"},
				OIDC: AuthOIDC{
					ClientID:     "test-client-id",
					ClientSecret: "test-secret",
					Issuer:       "https://auth.example.com",
					Scopes:       []string{"openid", "profile", "email"},
					AutoSignup:   true, // Defaults to true
					ButtonLabel:  "Login with SSO",
					RoleMapping:  OIDCRoleMapping{DefaultRole: "viewer"},
				},
				Builtin: AuthBuiltin{
					Admin: AdminConfig{Username: "admin"},
					Token: TokenConfig{TTL: 24 * time.Hour},
				},
			},
			TLS: &TLSConfig{
				CertFile: filepath.Join(testPaths, "cert.pem"),
				KeyFile:  filepath.Join(testPaths, "key.pem"),
			},
			Permissions:       map[Permission]bool{PermissionWriteDAGs: true, PermissionRunDAGs: true},
			LatestStatusToday: true,
			StrictValidation:  false,
			Metrics:           MetricsAccessPrivate,
			Terminal:          TerminalConfig{Enabled: true},
			Audit:             AuditConfig{Enabled: false, RetentionDays: 7},
		},
		Paths: PathsConfig{
			DAGsDir:            filepath.Join(testPaths, "dags"),
			Executable:         filepath.Join(testPaths, "bin", "dagu"),
			LogDir:             filepath.Join(testPaths, "logs"),
			DataDir:            filepath.Join(testPaths, "data"),
			SuspendFlagsDir:    filepath.Join(testPaths, "suspend"),
			AdminLogsDir:       filepath.Join(testPaths, "admin"),
			BaseConfig:         filepath.Join(testPaths, "base.yaml"),
			DAGRunsDir:         filepath.Join(testPaths, "runs"),
			ProcDir:            filepath.Join(testPaths, "proc"),
			QueueDir:           filepath.Join(testPaths, "queue"),
			ServiceRegistryDir: filepath.Join(testPaths, "service-registry"),
			UsersDir:           filepath.Join(testPaths, "data", "users"),                  // Derived from DataDir
			APIKeysDir:         filepath.Join(testPaths, "data", "apikeys"),                // Derived from DataDir
			WebhooksDir:        filepath.Join(testPaths, "data", "webhooks"),               // Derived from DataDir
			ConversationsDir:   filepath.Join(testPaths, "data", "agent", "conversations"), // Derived from DataDir
		},
		UI: UI{
			LogEncodingCharset:    "iso-8859-1",
			NavbarColor:           "#123456",
			NavbarTitle:           "Test Dagu",
			MaxDashboardPageLimit: 250,
			DAGs: DAGsConfig{
				SortField: "status",
				SortOrder: "desc",
			},
		},
		Queues: Queues{Enabled: false},
		Coordinator: Coordinator{
			Host:      "0.0.0.0",
			Advertise: "dagu-coordinator",
			Port:      50099,
		},
		Worker: Worker{
			ID:            "test-worker-123",
			MaxActiveRuns: 200,
			PostgresPool: PostgresPoolConfig{
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 300,
				ConnMaxIdleTime: 60,
			},
		},
		Scheduler: Scheduler{
			Port:                    9999,
			LockStaleThreshold:      30 * time.Second,
			LockRetryInterval:       5 * time.Second,
			ZombieDetectionInterval: 90 * time.Second,
		},
		Monitoring: MonitoringConfig{
			Retention: 24 * time.Hour,
			Interval:  5 * time.Second,
		},
		Tunnel: TunnelConfig{
			Enabled:       true,
			AllowTerminal: true,
			Tailscale: TailscaleTunnelConfig{
				AuthKey:  "tskey-test-123",
				Hostname: "test-dagu",
				Funnel:   false,
				HTTPS:    true,
			},
			RateLimiting: TunnelRateLimitConfig{
				Enabled:              true,
				LoginAttempts:        10,
				WindowSeconds:        600,
				BlockDurationSeconds: 1800,
			},
		},
		Warnings: []string{"Auth mode auto-detected as 'oidc' based on OIDC configuration (issuer: https://auth.example.com)"},
		Cache:    CacheModeNormal,
	}

	assert.Equal(t, expected, cfg)
}

func TestLoad_WithAppHomeDir(t *testing.T) {
	tempDir := t.TempDir()
	cfg := testLoad(t, WithAppHomeDir(tempDir))

	assert.Equal(t, filepath.Join(tempDir, "dags"), cfg.Paths.DAGsDir)
	assert.Equal(t, filepath.Join(tempDir, "data"), cfg.Paths.DataDir)
	assert.Equal(t, filepath.Join(tempDir, "logs"), cfg.Paths.LogDir)

	baseEnv := cfg.Core.BaseEnv.AsSlice()
	require.Contains(t, baseEnv, fmt.Sprintf("DAGU_HOME=%s", tempDir))
}

func TestLoad_YAML(t *testing.T) {
	cfg := loadFromYAML(t, `
host: "0.0.0.0"
port: 9090
permissions:
  writeDAGs: false
  runDAGs: false
debug: true
basePath: "/dagu"
apiBasePath: "/api/v1"
tz: "UTC"
logFormat: "json"
headless: true
latestStatusToday: true
defaultShell: "/bin/bash"
skipExamples: true
paths:
  dagsDir: "/var/dagu/dags"
  logDir: "/var/dagu/logs"
  dataDir: "/var/dagu/data"
  suspendFlagsDir: "/var/dagu/suspend"
  adminLogsDir: "/var/dagu/adminlogs"
  baseConfig: "/var/dagu/base.yaml"
  executable: "/usr/local/bin/dagu"
ui:
  navbarTitle: "Test Dagu"
  navbarColor: "#ff5733"
  logEncodingCharset: "iso-8859-1"
  maxDashboardPageLimit: 50
  dags:
    sortField: "name"
    sortOrder: "asc"
auth:
  basic:
    username: "admin"
    password: "secret"
  token:
    value: "api-token"
  oidc:
    clientId: "test-client-id"
    clientSecret: "test-client-secret"
    clientUrl: "http://localhost:8081"
    issuer: "https://accounts.example.com"
    scopes:
      - "openid"
      - "profile"
      - "email"
    whitelist:
      - "user@example.com"
remoteNodes:
  - name: "node1"
    apiBaseURL: "http://node1.example.com/api"
    isBasicAuth: true
    basicAuthUsername: "nodeuser"
    basicAuthPassword: "nodepass"
    skipTLSVerify: true
  - name: "node2"
    apiBaseURL: "http://node2.example.com/api"
    isAuthToken: true
    authToken: "node-token-123"
tls:
  certFile: "/path/to/cert.pem"
  keyFile: "/path/to/key.pem"
  caFile: "/path/to/ca.pem"
peer:
  certFile: "/path/to/peer-cert.pem"
  keyFile: "/path/to/peer-key.pem"
  clientCaFile: "/path/to/peer-ca.pem"
  skipTLSVerify: false
  insecure: false
queues:
  enabled: true
  config:
    - name: "critical"
      maxActiveRuns: 5
    - name: "normal"
      maxActiveRuns: 10
coordinator:
  host: "coordinator.example.com"
  port: 8081
worker:
  id: "worker-1"
  maxActiveRuns: 50
  labels:
    env: "production"
    region: "us-west-2"
scheduler:
  port: 7890
  lockStaleThreshold: 50s
  lockRetryInterval: 10s
  zombieDetectionInterval: 60s
`)

	utcLoc, _ := time.LoadLocation("UTC")

	expected := &Config{
		Core: Core{
			Debug:         true,
			LogFormat:     "json",
			TZ:            "UTC",
			TzOffsetInSec: 0,
			Location:      utcLoc,
			DefaultShell:  "/bin/bash",
			SkipExamples:  true,
			Peer: Peer{
				CertFile:      "/path/to/peer-cert.pem",
				KeyFile:       "/path/to/peer-key.pem",
				ClientCaFile:  "/path/to/peer-ca.pem",
				SkipTLSVerify: false,
				Insecure:      false,
			},
			BaseEnv: cfg.Core.BaseEnv, // Dynamic, copy from actual
		},
		Server: Server{
			Host:              "0.0.0.0",
			Port:              9090,
			BasePath:          "/dagu",
			APIBasePath:       "/api/v1",
			Headless:          true,
			LatestStatusToday: true,
			Auth: Auth{
				Mode:  AuthModeOIDC, // Auto-detected from OIDC config
				Basic: AuthBasic{Username: "admin", Password: "secret"},
				Token: AuthToken{Value: "api-token"},
				OIDC: AuthOIDC{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					ClientURL:    "http://localhost:8081",
					Issuer:       "https://accounts.example.com",
					Scopes:       []string{"openid", "profile", "email"},
					Whitelist:    []string{"user@example.com"},
					AutoSignup:   true, // Defaults to true
					ButtonLabel:  "Login with SSO",
					RoleMapping:  OIDCRoleMapping{DefaultRole: "viewer"},
				},
				Builtin: AuthBuiltin{
					Admin: AdminConfig{Username: "admin"},
					Token: TokenConfig{TTL: 24 * time.Hour},
				},
			},
			TLS: &TLSConfig{
				CertFile: "/path/to/cert.pem",
				KeyFile:  "/path/to/key.pem",
				CAFile:   "/path/to/ca.pem",
			},
			RemoteNodes: []RemoteNode{
				{
					Name:              "node1",
					APIBaseURL:        "http://node1.example.com/api",
					IsBasicAuth:       true,
					BasicAuthUsername: "nodeuser",
					BasicAuthPassword: "nodepass",
					SkipTLSVerify:     true,
				},
				{
					Name:        "node2",
					APIBaseURL:  "http://node2.example.com/api",
					IsAuthToken: true,
					AuthToken:   "node-token-123",
				},
			},
			Permissions: map[Permission]bool{
				PermissionWriteDAGs: false,
				PermissionRunDAGs:   false,
			},
			Metrics:  MetricsAccessPrivate,
			Terminal: TerminalConfig{Enabled: false},
			Audit:    AuditConfig{Enabled: true, RetentionDays: 7},
		},
		Paths: PathsConfig{
			DAGsDir:            "/var/dagu/dags",
			LogDir:             "/var/dagu/logs",
			DataDir:            "/var/dagu/data",
			SuspendFlagsDir:    "/var/dagu/suspend",
			AdminLogsDir:       "/var/dagu/adminlogs",
			BaseConfig:         "/var/dagu/base.yaml",
			Executable:         "/usr/local/bin/dagu",
			DAGRunsDir:         "/var/dagu/data/dag-runs",
			ProcDir:            "/var/dagu/data/proc",
			QueueDir:           "/var/dagu/data/queue",
			ServiceRegistryDir: "/var/dagu/data/service-registry",
			UsersDir:           "/var/dagu/data/users",
			APIKeysDir:         "/var/dagu/data/apikeys",
			WebhooksDir:        "/var/dagu/data/webhooks",
			ConversationsDir:   "/var/dagu/data/agent/conversations",
		},
		UI: UI{
			LogEncodingCharset:    "iso-8859-1",
			NavbarColor:           "#ff5733",
			NavbarTitle:           "Test Dagu",
			MaxDashboardPageLimit: 50,
			DAGs: DAGsConfig{
				SortField: "name",
				SortOrder: "asc",
			},
		},
		Queues: Queues{
			Enabled: true,
			Config: []QueueConfig{
				{Name: "critical", MaxActiveRuns: 5},
				{Name: "normal", MaxActiveRuns: 10},
			},
		},
		Coordinator: Coordinator{
			Host: "coordinator.example.com",
			Port: 8081,
		},
		Worker: Worker{
			ID:            "worker-1",
			MaxActiveRuns: 50,
			Labels: map[string]string{
				"env":    "production",
				"region": "us-west-2",
			},
			PostgresPool: PostgresPoolConfig{
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 300,
				ConnMaxIdleTime: 60,
			},
		},
		Scheduler: Scheduler{
			Port:                    7890,
			LockStaleThreshold:      50 * time.Second,
			LockRetryInterval:       10 * time.Second,
			ZombieDetectionInterval: 60 * time.Second,
		},
		Monitoring: MonitoringConfig{
			Retention: 24 * time.Hour,
			Interval:  5 * time.Second,
		},
		Warnings: []string{"Auth mode auto-detected as 'oidc' based on OIDC configuration (issuer: https://accounts.example.com)"},
		Cache:    CacheModeNormal,
	}

	assert.Equal(t, expected, cfg)
}

func TestLoad_EdgeCases_WorkerLabels(t *testing.T) {
	t.Run("LabelsFromString", func(t *testing.T) {
		cfg := loadFromYAML(t, `
worker:
  labels: "gpu=true,memory=64G,region=us-east-1"
`)
		assert.Equal(t, map[string]string{
			"gpu":    "true",
			"memory": "64G",
			"region": "us-east-1",
		}, cfg.Worker.Labels)
	})

	t.Run("LabelsFromMap", func(t *testing.T) {
		cfg := loadFromYAML(t, `
worker:
  labels:
    gpu: "true"
    memory: "64G"
    region: "us-west-2"
`)
		assert.Equal(t, map[string]string{
			"gpu":    "true",
			"memory": "64G",
			"region": "us-west-2",
		}, cfg.Worker.Labels)
	})

	t.Run("LabelsFromEnvironment", func(t *testing.T) {
		cfg := loadWithEnv(t, "# empty", map[string]string{
			"DAGU_WORKER_LABELS": "instance-type=m5.xlarge,cpu-arch=amd64",
		})
		assert.Equal(t, map[string]string{
			"instance-type": "m5.xlarge",
			"cpu-arch":      "amd64",
		}, cfg.Worker.Labels)
	})
}

func TestLoad_EdgeCases_DerivedPaths(t *testing.T) {
	cfg := loadFromYAML(t, `
paths:
  dataDir: "/custom/data"
`)
	assert.Equal(t, "/custom/data", cfg.Paths.DataDir)
	assert.Equal(t, "/custom/data/dag-runs", cfg.Paths.DAGRunsDir)
	assert.Equal(t, "/custom/data/proc", cfg.Paths.ProcDir)
	assert.Equal(t, "/custom/data/queue", cfg.Paths.QueueDir)
	assert.Equal(t, "/custom/data/service-registry", cfg.Paths.ServiceRegistryDir)
	assert.Equal(t, "/custom/data/users", cfg.Paths.UsersDir)
}

func TestLoad_EdgeCases_Errors(t *testing.T) {
	t.Run("InvalidTimezone", func(t *testing.T) {
		err := loadWithErrorFromYAML(t, `tz: "Invalid/Timezone"`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load timezone")
	})

	t.Run("IncompleteTLS", func(t *testing.T) {
		err := loadWithErrorFromYAML(t, `
tls:
  certFile: "/path/to/cert.pem"
  keyFile: ""
`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TLS configuration incomplete")
	})

	t.Run("InvalidPort_Negative", func(t *testing.T) {
		err := loadWithErrorFromYAML(t, `port: -1`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("InvalidPort_TooLarge", func(t *testing.T) {
		err := loadWithErrorFromYAML(t, `port: 99999`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("InvalidMaxDashboardPageLimit", func(t *testing.T) {
		err := loadWithErrorFromYAML(t, `
ui:
  maxDashboardPageLimit: 0
`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid max dashboard page limit")
	})

	t.Run("InvalidSchedulerDurations", func(t *testing.T) {
		cfg := loadFromYAML(t, `
scheduler:
  lockStaleThreshold: "invalid"
  lockRetryInterval: "bad-duration"
  zombieDetectionInterval: "not-a-duration"
`)
		assert.Equal(t, 30*time.Second, cfg.Scheduler.LockStaleThreshold)
		assert.Equal(t, 5*time.Second, cfg.Scheduler.LockRetryInterval)
		assert.Equal(t, time.Duration(0), cfg.Scheduler.ZombieDetectionInterval)

		require.Len(t, cfg.Warnings, 3)
		assert.Contains(t, cfg.Warnings[0], "Invalid scheduler.lockStaleThreshold")
		assert.Contains(t, cfg.Warnings[1], "Invalid scheduler.lockRetryInterval")
		assert.Contains(t, cfg.Warnings[2], "Invalid scheduler.zombieDetectionInterval")
	})

	t.Run("BuiltinAuthWithBasicAuthWarning", func(t *testing.T) {
		t.Parallel()
		cfg := loadWithEnv(t, `
auth:
  mode: builtin
  builtin:
    admin:
      username: admin
    token:
      secret: test-secret
  basic:
    username: basicuser
    password: basicpass
paths:
  usersDir: /tmp/users
`, nil)
		require.Len(t, cfg.Warnings, 1)
		assert.Contains(t, cfg.Warnings[0], "Basic auth configuration is ignored when auth mode is 'builtin'")
	})
}

func TestLoad_LegacyEnv(t *testing.T) {
	tempDir := t.TempDir()

	cfg := loadWithEnv(t, "# empty", map[string]string{
		"DAGU__ADMIN_PORT":         "1234",
		"DAGU__ADMIN_HOST":         "0.0.0.0",
		"DAGU__ADMIN_NAVBAR_TITLE": "LegacyTitle",
		"DAGU__ADMIN_NAVBAR_COLOR": "#abc123",
		"DAGU__DATA":               filepath.Join(tempDir, "legacy", "data"),
		"DAGU__SUSPEND_FLAGS_DIR":  filepath.Join(tempDir, "legacy", "suspend"),
		"DAGU__ADMIN_LOGS_DIR":     filepath.Join(tempDir, "legacy", "adminlogs"),
	})

	assert.Equal(t, 1234, cfg.Server.Port)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, "LegacyTitle", cfg.UI.NavbarTitle)
	assert.Equal(t, "#abc123", cfg.UI.NavbarColor)
	assert.Equal(t, filepath.Join(tempDir, "legacy", "data"), cfg.Paths.DataDir)
	assert.Equal(t, filepath.Join(tempDir, "legacy", "suspend"), cfg.Paths.SuspendFlagsDir)
	assert.Equal(t, filepath.Join(tempDir, "legacy", "adminlogs"), cfg.Paths.AdminLogsDir)
}

func TestLoad_LoadLegacyFields(t *testing.T) {
	t.Parallel()
	loader := &ConfigLoader{}

	t.Run("AllFieldsSet", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		testPaths := filepath.Join(tempDir, "test")

		def := Definition{
			BasicAuthUsername:     "user",
			BasicAuthPassword:     "pass",
			APIBaseURL:            "/api/v1",
			IsAuthToken:           true,
			AuthToken:             "token123",
			DAGs:                  filepath.Join(testPaths, "legacy", "dags"),
			DAGsDir:               filepath.Join(testPaths, "new", "dags"), // Takes precedence over DAGs
			Executable:            filepath.Join(testPaths, "bin", "dagu"),
			LogDir:                filepath.Join(testPaths, "logs"),
			DataDir:               filepath.Join(testPaths, "data"),
			SuspendFlagsDir:       filepath.Join(testPaths, "suspend"),
			AdminLogsDir:          filepath.Join(testPaths, "adminlogs"),
			BaseConfig:            filepath.Join(testPaths, "base.yaml"),
			LogEncodingCharset:    "iso-8859-1",
			NavbarColor:           "#123456",
			NavbarTitle:           "Title",
			MaxDashboardPageLimit: 100,
		}

		cfg := Config{}
		err := loader.LoadLegacyFields(&cfg, def)
		require.NoError(t, err)

		// Auth
		assert.Equal(t, "user", cfg.Server.Auth.Basic.Username)
		assert.Equal(t, "pass", cfg.Server.Auth.Basic.Password)
		assert.Equal(t, "token123", cfg.Server.Auth.Token.Value)
		assert.Equal(t, "/api/v1", cfg.Server.APIBasePath)

		// Paths - DAGsDir should take precedence over DAGs
		assert.Equal(t, filepath.Join(testPaths, "new", "dags"), cfg.Paths.DAGsDir)
		assert.Equal(t, filepath.Join(testPaths, "bin", "dagu"), cfg.Paths.Executable)
		assert.Equal(t, filepath.Join(testPaths, "logs"), cfg.Paths.LogDir)
		assert.Equal(t, filepath.Join(testPaths, "data"), cfg.Paths.DataDir)
		assert.Equal(t, filepath.Join(testPaths, "suspend"), cfg.Paths.SuspendFlagsDir)
		assert.Equal(t, filepath.Join(testPaths, "adminlogs"), cfg.Paths.AdminLogsDir)
		assert.Equal(t, filepath.Join(testPaths, "base.yaml"), cfg.Paths.BaseConfig)

		// UI
		assert.Equal(t, "iso-8859-1", cfg.UI.LogEncodingCharset)
		assert.Equal(t, "#123456", cfg.UI.NavbarColor)
		assert.Equal(t, "Title", cfg.UI.NavbarTitle)
		assert.Equal(t, 100, cfg.UI.MaxDashboardPageLimit)
	})

	t.Run("DAGsPrecedence", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()

		def := Definition{
			DAGs:    filepath.Join(tempDir, "legacy", "dags"),
			DAGsDir: filepath.Join(tempDir, "new", "dags"),
		}
		cfg := Config{}
		err := loader.LoadLegacyFields(&cfg, def)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tempDir, "new", "dags"), cfg.Paths.DAGsDir)

		def2 := Definition{
			DAGs: filepath.Join(tempDir, "legacy", "dags"),
		}
		cfg2 := Config{}
		err = loader.LoadLegacyFields(&cfg2, def2)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tempDir, "legacy", "dags"), cfg2.Paths.DAGsDir)
	})
}

func loadWithEnv(t *testing.T, yamlContent string, env map[string]string) *Config {
	t.Helper()
	viper.Reset()

	for k, v := range env {
		t.Setenv(k, v)
	}

	return loadFromYAML(t, yamlContent)
}

func loadFromYAML(t *testing.T, yamlContent string) *Config {
	t.Helper()
	viper.Reset()

	configFile := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(configFile, []byte(yamlContent), 0600)
	require.NoError(t, err)

	cfg := testLoad(t, WithConfigFile(configFile))
	cfg.Paths.ConfigFileUsed = ""
	return cfg
}

func loadWithErrorFromYAML(t *testing.T, yamlContent string) error {
	t.Helper()
	viper.Reset()

	configFile := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(configFile, []byte(yamlContent), 0600)
	require.NoError(t, err)

	return testLoadWithError(t, WithConfigFile(configFile))
}

func TestLoad_ConfigFileUsed(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("host: 127.0.0.1"), 0600)
	require.NoError(t, err)

	cfg := testLoad(t, WithConfigFile(configFile))

	assert.Equal(t, configFile, cfg.Paths.ConfigFileUsed)
}

func TestLoad_ConfigFileUsed_RelativePath(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("host: 127.0.0.1"), 0600)
	require.NoError(t, err)

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(origDir)
	}()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	cfg := testLoad(t, WithConfigFile("config.yaml"))

	assert.True(t, filepath.IsAbs(cfg.Paths.ConfigFileUsed),
		"ConfigFileUsed should be absolute, got: %s", cfg.Paths.ConfigFileUsed)

	expectedPath, err := filepath.EvalSymlinks(configFile)
	require.NoError(t, err)
	actualPath, err := filepath.EvalSymlinks(cfg.Paths.ConfigFileUsed)
	require.NoError(t, err)
	assert.Equal(t, expectedPath, actualPath)
}

func TestBindEnv_AsPath(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		wantAbs  bool
	}{
		{
			name:     "relative path gets resolved to absolute",
			envValue: "relative/path",
			wantAbs:  true,
		},
		{
			name:     "absolute path stays absolute",
			envValue: filepath.Join(os.TempDir(), "absolute/path"),
			wantAbs:  true,
		},
		{
			name:     "empty value stays empty",
			envValue: "",
			wantAbs:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const envKey = "DAGU_DAGS_DIR"
			t.Setenv(envKey, tt.envValue)

			loader := NewConfigLoader(viper.New())
			loader.bindEnvironmentVariables()

			result := os.Getenv(envKey)
			if tt.envValue == "" {
				assert.Empty(t, result)
			} else if tt.wantAbs {
				assert.True(t, filepath.IsAbs(result), "expected absolute path, got: %s", result)
			}
		})
	}
}

func TestLoad_Monitoring(t *testing.T) {
	t.Run("FromYAML", func(t *testing.T) {
		cfg := loadFromYAML(t, `
monitoring:
  retention: "24h"
  interval: "10s"
`)
		assert.Equal(t, 24*time.Hour, cfg.Monitoring.Retention)
		assert.Equal(t, 10*time.Second, cfg.Monitoring.Interval)
	})

	t.Run("FromEnv", func(t *testing.T) {
		cfg := loadWithEnv(t, "# empty", map[string]string{
			"DAGU_MONITORING_RETENTION": "30m",
			"DAGU_MONITORING_INTERVAL":  "15s",
		})
		assert.Equal(t, 30*time.Minute, cfg.Monitoring.Retention)
		assert.Equal(t, 15*time.Second, cfg.Monitoring.Interval)
	})

	t.Run("Default", func(t *testing.T) {
		cfg := loadFromYAML(t, "")
		assert.Equal(t, 24*time.Hour, cfg.Monitoring.Retention)
		assert.Equal(t, 5*time.Second, cfg.Monitoring.Interval)
	})
}

func TestLoad_AuthMode(t *testing.T) {
	t.Run("AuthModeNone", func(t *testing.T) {
		cfg := loadFromYAML(t, `
auth:
  mode: "none"
`)
		assert.Equal(t, AuthModeNone, cfg.Server.Auth.Mode)
	})

	t.Run("AuthModeBuiltin", func(t *testing.T) {
		cfg := loadFromYAML(t, `
auth:
  mode: "builtin"
  builtin:
    admin:
      username: "admin"
      password: "secretpass123"
    token:
      secret: "my-jwt-secret-key"
      ttl: "12h"
`)
		assert.Equal(t, AuthModeBuiltin, cfg.Server.Auth.Mode)
		assert.Equal(t, "admin", cfg.Server.Auth.Builtin.Admin.Username)
		assert.Equal(t, "secretpass123", cfg.Server.Auth.Builtin.Admin.Password)
		assert.Equal(t, "my-jwt-secret-key", cfg.Server.Auth.Builtin.Token.Secret)
		assert.Equal(t, 12*time.Hour, cfg.Server.Auth.Builtin.Token.TTL)
	})

	t.Run("AuthModeOIDC", func(t *testing.T) {
		cfg := loadFromYAML(t, `
auth:
  mode: "oidc"
  oidc:
    clientId: "my-client-id"
    clientSecret: "my-client-secret"
    issuer: "https://auth.example.com"
    scopes:
      - "openid"
      - "profile"
`)
		assert.Equal(t, AuthModeOIDC, cfg.Server.Auth.Mode)
		assert.Equal(t, "my-client-id", cfg.Server.Auth.OIDC.ClientID)
		assert.Equal(t, "my-client-secret", cfg.Server.Auth.OIDC.ClientSecret)
		assert.Equal(t, "https://auth.example.com", cfg.Server.Auth.OIDC.Issuer)
		assert.Equal(t, []string{"openid", "profile"}, cfg.Server.Auth.OIDC.Scopes)
	})

	t.Run("AuthModeFromEnv", func(t *testing.T) {
		cfg := loadWithEnv(t, "# empty", map[string]string{
			"DAGU_AUTH_MODE":         "builtin",
			"DAGU_AUTH_TOKEN_SECRET": "test-secret",
			"DAGU_PATHS_USERS_DIR":   t.TempDir(),
		})
		assert.Equal(t, AuthModeBuiltin, cfg.Server.Auth.Mode)
	})

	t.Run("AuthModeDefaultNone", func(t *testing.T) {
		cfg := loadFromYAML(t, "# empty")
		assert.Equal(t, AuthModeNone, cfg.Server.Auth.Mode)
	})

	t.Run("AuthModeInvalid", func(t *testing.T) {
		cfg := loadFromYAML(t, `
auth:
  mode: "invalid_mode"
`)
		assert.Equal(t, AuthModeNone, cfg.Server.Auth.Mode)
		require.Len(t, cfg.Warnings, 1)
		assert.Contains(t, cfg.Warnings[0], "Invalid auth.mode value")
		assert.Contains(t, cfg.Warnings[0], "invalid_mode")
	})
}

func TestLoad_AuthBuiltin(t *testing.T) {
	t.Run("FromYAML", func(t *testing.T) {
		cfg := loadFromYAML(t, `
auth:
  mode: "builtin"
  builtin:
    admin:
      username: "superadmin"
      password: "supersecret123"
    token:
      secret: "jwt-signing-secret"
      ttl: "24h"
`)
		assert.Equal(t, AuthModeBuiltin, cfg.Server.Auth.Mode)
		assert.Equal(t, "superadmin", cfg.Server.Auth.Builtin.Admin.Username)
		assert.Equal(t, "supersecret123", cfg.Server.Auth.Builtin.Admin.Password)
		assert.Equal(t, "jwt-signing-secret", cfg.Server.Auth.Builtin.Token.Secret)
		assert.Equal(t, 24*time.Hour, cfg.Server.Auth.Builtin.Token.TTL)
	})

	t.Run("FromEnv", func(t *testing.T) {
		cfg := loadWithEnv(t, "# empty", map[string]string{
			"DAGU_AUTH_MODE":           "builtin",
			"DAGU_AUTH_ADMIN_USERNAME": "envadmin",
			"DAGU_AUTH_ADMIN_PASSWORD": "envpassword123",
			"DAGU_AUTH_TOKEN_SECRET":   "env-jwt-secret",
			"DAGU_AUTH_TOKEN_TTL":      "48h",
		})
		assert.Equal(t, AuthModeBuiltin, cfg.Server.Auth.Mode)
		assert.Equal(t, "envadmin", cfg.Server.Auth.Builtin.Admin.Username)
		assert.Equal(t, "envpassword123", cfg.Server.Auth.Builtin.Admin.Password)
		assert.Equal(t, "env-jwt-secret", cfg.Server.Auth.Builtin.Token.Secret)
		assert.Equal(t, 48*time.Hour, cfg.Server.Auth.Builtin.Token.TTL)
	})

	t.Run("EmptyPasswordAllowed", func(t *testing.T) {
		cfg := loadFromYAML(t, `
auth:
  mode: "builtin"
  builtin:
    admin:
      username: "admin"
      password: ""
    token:
      secret: "secret"
      ttl: "1h"
`)
		assert.Equal(t, "admin", cfg.Server.Auth.Builtin.Admin.Username)
		assert.Equal(t, "", cfg.Server.Auth.Builtin.Admin.Password)
	})

	t.Run("DefaultTTL", func(t *testing.T) {
		cfg := loadFromYAML(t, `
auth:
  mode: "builtin"
  builtin:
    admin:
      username: "admin"
    token:
      secret: "secret"
`)
		assert.Equal(t, 24*time.Hour, cfg.Server.Auth.Builtin.Token.TTL)
	})
}

func TestLoad_MetricsAccess(t *testing.T) {
	t.Run("MetricsAccessPublic", func(t *testing.T) {
		cfg := loadFromYAML(t, `
metrics: "public"
`)
		assert.Equal(t, MetricsAccessPublic, cfg.Server.Metrics)
	})

	t.Run("MetricsAccessPrivate", func(t *testing.T) {
		cfg := loadFromYAML(t, `
metrics: "private"
`)
		assert.Equal(t, MetricsAccessPrivate, cfg.Server.Metrics)
	})

	t.Run("MetricsAccessDefault", func(t *testing.T) {
		cfg := loadFromYAML(t, "# empty")
		assert.Equal(t, MetricsAccessPrivate, cfg.Server.Metrics)
	})

	t.Run("MetricsAccessFromEnv", func(t *testing.T) {
		cfg := loadWithEnv(t, "# empty", map[string]string{
			"DAGU_SERVER_METRICS": "public",
		})
		assert.Equal(t, MetricsAccessPublic, cfg.Server.Metrics)
	})

	t.Run("MetricsAccessInvalid", func(t *testing.T) {
		cfg := loadFromYAML(t, `
metrics: "invalid_value"
`)
		assert.Equal(t, MetricsAccessPrivate, cfg.Server.Metrics)
		require.Len(t, cfg.Warnings, 1)
		assert.Contains(t, cfg.Warnings[0], "Invalid server.metrics value")
		assert.Contains(t, cfg.Warnings[0], "invalid_value")
	})
}

func TestLoad_CacheConfig(t *testing.T) {
	t.Run("DefaultCacheMode", func(t *testing.T) {
		cfg := loadFromYAML(t, ``)
		assert.Equal(t, CacheModeNormal, cfg.Cache)
	})

	t.Run("CacheModeLow", func(t *testing.T) {
		cfg := loadFromYAML(t, `
cache: low
`)
		assert.Equal(t, CacheModeLow, cfg.Cache)
	})

	t.Run("CacheModeNormal", func(t *testing.T) {
		cfg := loadFromYAML(t, `
cache: normal
`)
		assert.Equal(t, CacheModeNormal, cfg.Cache)
	})

	t.Run("CacheModeHigh", func(t *testing.T) {
		cfg := loadFromYAML(t, `
cache: high
`)
		assert.Equal(t, CacheModeHigh, cfg.Cache)
	})

	t.Run("CacheModeInvalid", func(t *testing.T) {
		cfg := loadFromYAML(t, `
cache: invalid
`)
		assert.Equal(t, CacheModeNormal, cfg.Cache)
		require.Len(t, cfg.Warnings, 1)
		assert.Contains(t, cfg.Warnings[0], "Invalid cache mode")
	})

	t.Run("CacheModeFromEnv", func(t *testing.T) {
		cfg := loadWithEnv(t, ``, map[string]string{
			"DAGU_CACHE": "low",
		})
		assert.Equal(t, CacheModeLow, cfg.Cache)
	})
}

func TestParseCoordinatorAddresses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		input            interface{}
		expectedAddrs    []string
		expectedWarnings int // Number of expected warnings
	}{
		// Valid string input tests
		{"empty string", "", nil, 0},
		{"single address string", "host:8080", []string{"host:8080"}, 0},
		{"multiple addresses string", "host1:8080,host2:9090", []string{"host1:8080", "host2:9090"}, 0},
		{"whitespace trimmed string", " host1:8080 , host2:9090 ", []string{"host1:8080", "host2:9090"}, 0},
		{"empty parts filtered", "host1:8080,,host2:9090", []string{"host1:8080", "host2:9090"}, 0},
		{"trailing comma", "host:8080,", []string{"host:8080"}, 0},
		{"leading comma", ",host:8080", []string{"host:8080"}, 0},
		{"IPv6 address", "[::1]:8080", []string{"[::1]:8080"}, 0},

		// Valid []interface{} input tests (YAML)
		{"empty interface slice", []interface{}{}, nil, 0},
		{"interface slice with addresses", []interface{}{"host1:8080", "host2:9090"}, []string{"host1:8080", "host2:9090"}, 0},
		{"interface slice filters non-strings", []interface{}{"host:8080", 123, "host2:9090"}, []string{"host:8080", "host2:9090"}, 0},
		{"interface slice filters empty strings", []interface{}{"host:8080", "", "host2:9090"}, []string{"host:8080", "host2:9090"}, 0},
		{"interface slice trims whitespace", []interface{}{" host1:8080 ", " host2:9090 "}, []string{"host1:8080", "host2:9090"}, 0},

		// Valid []string input tests
		{"empty string slice", []string{}, nil, 0},
		{"string slice with addresses", []string{"host1:8080", "host2:9090"}, []string{"host1:8080", "host2:9090"}, 0},
		{"string slice trims whitespace", []string{" host1:8080 ", " host2:9090 "}, []string{"host1:8080", "host2:9090"}, 0},
		{"string slice filters empty", []string{"host:8080", "", "host2:9090"}, []string{"host:8080", "host2:9090"}, 0},

		// Edge cases
		{"nil input", nil, nil, 0},
		{"unsupported type int", 12345, nil, 0},
		{"unsupported type map", map[string]string{"key": "value"}, nil, 0},

		// Invalid addresses - should generate warnings
		{"missing port", "host", nil, 1},
		{"invalid port", "host:abc", nil, 1},
		{"port out of range", "host:70000", nil, 1},
		{"port zero", "host:0", nil, 1},
		{"empty host", ":8080", nil, 1},
		{"mixed valid and invalid", "host1:8080,invalid,host2:9090", []string{"host1:8080", "host2:9090"}, 1},
		{"all invalid", []string{"invalid1", "invalid2"}, nil, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, warnings := parseCoordinatorAddresses(tt.input)
			assert.Equal(t, tt.expectedAddrs, result)
			assert.Len(t, warnings, tt.expectedWarnings)
		})
	}
}

func TestLoad_Terminal(t *testing.T) {
	t.Run("TerminalDefault", func(t *testing.T) {
		cfg := loadFromYAML(t, "# empty")
		assert.False(t, cfg.Server.Terminal.Enabled)
	})

	t.Run("TerminalEnabled", func(t *testing.T) {
		cfg := loadFromYAML(t, `
terminal:
  enabled: true
`)
		assert.True(t, cfg.Server.Terminal.Enabled)
	})

	t.Run("TerminalDisabled", func(t *testing.T) {
		cfg := loadFromYAML(t, `
terminal:
  enabled: false
`)
		assert.False(t, cfg.Server.Terminal.Enabled)
	})

	t.Run("TerminalEnabledFromEnv", func(t *testing.T) {
		cfg := loadWithEnv(t, "# empty", map[string]string{
			"DAGU_TERMINAL_ENABLED": "true",
		})
		assert.True(t, cfg.Server.Terminal.Enabled)
	})

	t.Run("TerminalDisabledFromEnv", func(t *testing.T) {
		cfg := loadWithEnv(t, "# empty", map[string]string{
			"DAGU_TERMINAL_ENABLED": "false",
		})
		assert.False(t, cfg.Server.Terminal.Enabled)
	})

	t.Run("TerminalEnvOverridesYAML", func(t *testing.T) {
		cfg := loadWithEnv(t, `
terminal:
  enabled: false
`, map[string]string{
			"DAGU_TERMINAL_ENABLED": "true",
		})
		assert.True(t, cfg.Server.Terminal.Enabled)
	})
}

func TestLoad_Audit(t *testing.T) {
	t.Run("AuditDefault", func(t *testing.T) {
		cfg := loadFromYAML(t, "# empty")
		assert.True(t, cfg.Server.Audit.Enabled)
	})

	t.Run("AuditEnabled", func(t *testing.T) {
		cfg := loadFromYAML(t, `
audit:
  enabled: true
`)
		assert.True(t, cfg.Server.Audit.Enabled)
	})

	t.Run("AuditDisabled", func(t *testing.T) {
		cfg := loadFromYAML(t, `
audit:
  enabled: false
`)
		assert.False(t, cfg.Server.Audit.Enabled)
	})

	t.Run("AuditEnabledFromEnv", func(t *testing.T) {
		cfg := loadWithEnv(t, "# empty", map[string]string{
			"DAGU_AUDIT_ENABLED": "true",
		})
		assert.True(t, cfg.Server.Audit.Enabled)
	})

	t.Run("AuditDisabledFromEnv", func(t *testing.T) {
		cfg := loadWithEnv(t, "# empty", map[string]string{
			"DAGU_AUDIT_ENABLED": "false",
		})
		assert.False(t, cfg.Server.Audit.Enabled)
	})

	t.Run("AuditEnvOverridesYAML", func(t *testing.T) {
		cfg := loadWithEnv(t, `
audit:
  enabled: true
`, map[string]string{
			"DAGU_AUDIT_ENABLED": "false",
		})
		assert.False(t, cfg.Server.Audit.Enabled)
	})
}

func TestLoad_TunnelConfig(t *testing.T) {
	t.Run("TunnelDefault", func(t *testing.T) {
		cfg := loadFromYAML(t, "# empty")
		assert.False(t, cfg.Tunnel.Enabled)
		assert.Empty(t, cfg.Tunnel.Tailscale.AuthKey)
		assert.Empty(t, cfg.Tunnel.Tailscale.Hostname)
		assert.False(t, cfg.Tunnel.Tailscale.Funnel)
		assert.False(t, cfg.Tunnel.Tailscale.HTTPS)
	})

	t.Run("TunnelFromYAML", func(t *testing.T) {
		cfg := loadFromYAML(t, `
tunnel:
  enabled: true
  tailscale:
    authKey: "tskey-yaml-test"
    hostname: "yaml-dagu"
    funnel: false
    https: true
    stateDir: "/var/dagu/tailscale"
  allowTerminal: true
  allowedIPs:
    - "192.168.1.0/24"
    - "10.0.0.0/8"
  rateLimiting:
    enabled: true
    loginAttempts: 5
    windowSeconds: 300
    blockDurationSeconds: 900
`)
		assert.True(t, cfg.Tunnel.Enabled)
		assert.Equal(t, "tskey-yaml-test", cfg.Tunnel.Tailscale.AuthKey)
		assert.Equal(t, "yaml-dagu", cfg.Tunnel.Tailscale.Hostname)
		assert.False(t, cfg.Tunnel.Tailscale.Funnel)
		assert.True(t, cfg.Tunnel.Tailscale.HTTPS)
		assert.Equal(t, "/var/dagu/tailscale", cfg.Tunnel.Tailscale.StateDir)
		assert.True(t, cfg.Tunnel.AllowTerminal)
		assert.Equal(t, []string{"192.168.1.0/24", "10.0.0.0/8"}, cfg.Tunnel.AllowedIPs)
		assert.True(t, cfg.Tunnel.RateLimiting.Enabled)
		assert.Equal(t, 5, cfg.Tunnel.RateLimiting.LoginAttempts)
		assert.Equal(t, 300, cfg.Tunnel.RateLimiting.WindowSeconds)
		assert.Equal(t, 900, cfg.Tunnel.RateLimiting.BlockDurationSeconds)
	})

	t.Run("TunnelFromEnv", func(t *testing.T) {
		cfg := loadWithEnv(t, "# empty", map[string]string{
			"DAGU_TUNNEL_ENABLED":                              "true",
			"DAGU_TUNNEL_TAILSCALE_AUTH_KEY":                   "tskey-env-test",
			"DAGU_TUNNEL_TAILSCALE_HOSTNAME":                   "env-dagu",
			"DAGU_TUNNEL_TAILSCALE_FUNNEL":                     "false",
			"DAGU_TUNNEL_TAILSCALE_HTTPS":                      "true",
			"DAGU_TUNNEL_ALLOW_TERMINAL":                       "true",
			"DAGU_TUNNEL_RATE_LIMITING_ENABLED":                "true",
			"DAGU_TUNNEL_RATE_LIMITING_LOGIN_ATTEMPTS":         "10",
			"DAGU_TUNNEL_RATE_LIMITING_WINDOW_SECONDS":         "600",
			"DAGU_TUNNEL_RATE_LIMITING_BLOCK_DURATION_SECONDS": "1800",
		})
		assert.True(t, cfg.Tunnel.Enabled)
		assert.Equal(t, "tskey-env-test", cfg.Tunnel.Tailscale.AuthKey)
		assert.Equal(t, "env-dagu", cfg.Tunnel.Tailscale.Hostname)
		assert.False(t, cfg.Tunnel.Tailscale.Funnel)
		assert.True(t, cfg.Tunnel.Tailscale.HTTPS)
		assert.True(t, cfg.Tunnel.AllowTerminal)
		assert.True(t, cfg.Tunnel.RateLimiting.Enabled)
		assert.Equal(t, 10, cfg.Tunnel.RateLimiting.LoginAttempts)
		assert.Equal(t, 600, cfg.Tunnel.RateLimiting.WindowSeconds)
		assert.Equal(t, 1800, cfg.Tunnel.RateLimiting.BlockDurationSeconds)
	})

	t.Run("TunnelEnvOverridesYAML", func(t *testing.T) {
		cfg := loadWithEnv(t, `
tunnel:
  enabled: true
  tailscale:
    authKey: "yaml-key"
    hostname: "yaml-host"
    https: false
`, map[string]string{
			"DAGU_TUNNEL_TAILSCALE_AUTH_KEY": "env-key",
			"DAGU_TUNNEL_TAILSCALE_HTTPS":    "true",
		})
		assert.True(t, cfg.Tunnel.Enabled)
		assert.Equal(t, "env-key", cfg.Tunnel.Tailscale.AuthKey)
		assert.Equal(t, "yaml-host", cfg.Tunnel.Tailscale.Hostname)
		assert.True(t, cfg.Tunnel.Tailscale.HTTPS)
	})

	t.Run("TunnelDefaultHostname", func(t *testing.T) {
		cfg := loadFromYAML(t, `
tunnel:
  enabled: true
`)
		assert.True(t, cfg.Tunnel.Enabled)
		assert.Equal(t, AppSlug, cfg.Tunnel.Tailscale.Hostname)
	})

	t.Run("TunnelDefaultRateLimiting", func(t *testing.T) {
		cfg := loadFromYAML(t, `
tunnel:
  enabled: true
`)
		assert.Equal(t, 5, cfg.Tunnel.RateLimiting.LoginAttempts)
		assert.Equal(t, 300, cfg.Tunnel.RateLimiting.WindowSeconds)
		assert.Equal(t, 900, cfg.Tunnel.RateLimiting.BlockDurationSeconds)
	})

	t.Run("TunnelDisabledNoDefaults", func(t *testing.T) {
		cfg := loadFromYAML(t, `
tunnel:
  enabled: false
`)
		assert.False(t, cfg.Tunnel.Enabled)
		assert.Empty(t, cfg.Tunnel.Tailscale.Hostname)
	})
}
