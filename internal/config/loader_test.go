package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Env(t *testing.T) {
	// Reset viper to ensure clean state
	viper.Reset()
	defer viper.Reset()

	// Create a minimal config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	err := os.WriteFile(configFile, []byte("# minimal config"), 0600)
	require.NoError(t, err)

	// Define all environment variables that should be bound according to bindEnvironmentVariables()
	testEnvs := map[string]string{
		// Server configurations
		"DAGU_LOG_FORMAT":   "json",
		"DAGU_BASE_PATH":    "/test/base",
		"DAGU_API_BASE_URL": "/test/api",
		"DAGU_TZ":           "Europe/Berlin",
		"DAGU_HOST":         "test.example.com",
		"DAGU_PORT":         "9876",
		"DAGU_DEBUG":        "true",
		"DAGU_HEADLESS":     "true",

		// Global configurations
		"DAGU_WORK_DIR":      "/test/work",
		"DAGU_DEFAULT_SHELL": "/bin/zsh",

		// UI configurations (new keys)
		"DAGU_UI_MAX_DASHBOARD_PAGE_LIMIT": "250",
		"DAGU_UI_LOG_ENCODING_CHARSET":     "iso-8859-1",
		"DAGU_UI_NAVBAR_COLOR":             "#123456",
		"DAGU_UI_NAVBAR_TITLE":             "Test Dagu",

		// Authentication configurations (new keys)
		"DAGU_AUTH_BASIC_USERNAME": "testuser",
		"DAGU_AUTH_BASIC_PASSWORD": "testpass",
		"DAGU_AUTH_TOKEN":          "test-token-123",

		// TLS configurations
		"DAGU_CERT_FILE": "/test/cert.pem",
		"DAGU_KEY_FILE":  "/test/key.pem",

		// File paths
		"DAGU_DAGS_DIR":             "/test/dags",
		"DAGU_EXECUTABLE":           "/test/bin/dagu",
		"DAGU_LOG_DIR":              "/test/logs",
		"DAGU_DATA_DIR":             "/test/data",
		"DAGU_SUSPEND_FLAGS_DIR":    "/test/suspend",
		"DAGU_ADMIN_LOG_DIR":        "/test/admin",
		"DAGU_BASE_CONFIG":          "/test/base.yaml",
		"DAGU_DAG_RUNS_DIR":         "/test/runs",
		"DAGU_PROC_DIR":             "/test/proc",
		"DAGU_QUEUE_DIR":            "/test/queue",
		"DAGU_SERVICE_REGISTRY_DIR": "/test/service-registry",

		// UI customization
		"DAGU_LATEST_STATUS_TODAY": "true",

		// Queue configuration
		"DAGU_QUEUE_ENABLED": "false",

		// Worker configuration - env vars still bound but to nested structure
		"DAGU_WORKER_ID":              "test-worker-123",
		"DAGU_WORKER_MAX_ACTIVE_RUNS": "200",

		// Scheduler configuration
		"DAGU_SCHEDULER_PORT":                      "9999",
		"DAGU_SCHEDULER_ZOMBIE_DETECTION_INTERVAL": "90s",

		// OIDC Authentication configurations
		"DAGU_AUTH_OIDC_CLIENT_ID":     "test-client-id",
		"DAGU_AUTH_OIDC_CLIENT_SECRET": "test-secret",
		"DAGU_AUTH_OIDC_ISSUER":        "https://auth.example.com",
		"DAGU_AUTH_OIDC_SCOPES":        "openid,profile,email",

		// UI DAGs configuration
		"DAGU_UI_DAGS_SORT_FIELD": "status",
		"DAGU_UI_DAGS_SORT_ORDER": "desc",
	}

	// Save and clear existing environment variables
	savedEnvs := make(map[string]string)
	for key := range testEnvs {
		savedEnvs[key] = os.Getenv(key)
		os.Unsetenv(key)
	}
	defer func() {
		// Restore original environment
		for key, val := range savedEnvs {
			if val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Set test environment variables
	for key, val := range testEnvs {
		os.Setenv(key, val)
	}

	cfg, err := config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)

	berlinLoc, _ := time.LoadLocation("Europe/Berlin")
	_, berlinOffset := time.Now().In(berlinLoc).Zone()

	expected := &config.Config{
		Global: config.Global{
			Debug:         true,
			LogFormat:     "json",
			TZ:            "Europe/Berlin",
			TzOffsetInSec: berlinOffset,
			Location:      berlinLoc,
			DefaultShell:  "/bin/zsh",
			SkipExamples:  false,
			Peer:          config.Peer{Insecure: true}, // Default is true
			BaseEnv:       cfg.Global.BaseEnv,          // Dynamic, copy from actual
		},
		Server: config.Server{
			Host:        "test.example.com",
			Port:        9876,
			BasePath:    "/test/base",
			APIBasePath: "/test/api",
			Headless:    true,
			Auth: config.Auth{
				Basic: config.AuthBasic{Username: "testuser", Password: "testpass"},
				Token: config.AuthToken{Value: "test-token-123"},
				OIDC: config.AuthOIDC{
					ClientId:     "test-client-id",
					ClientSecret: "test-secret",
					Issuer:       "https://auth.example.com",
					Scopes:       []string{"openid", "profile", "email"},
				},
			},
			TLS: &config.TLSConfig{
				CertFile: "/test/cert.pem",
				KeyFile:  "/test/key.pem",
			},
			Permissions:       map[config.Permission]bool{config.PermissionWriteDAGs: true, config.PermissionRunDAGs: true},
			LatestStatusToday: true,
			StrictValidation:  false,
		},
		Paths: config.PathsConfig{
			DAGsDir:            "/test/dags",
			Executable:         "/test/bin/dagu",
			LogDir:             "/test/logs",
			DataDir:            "/test/data",
			SuspendFlagsDir:    "/test/suspend",
			AdminLogsDir:       "/test/admin",
			BaseConfig:         "/test/base.yaml",
			DAGRunsDir:         "/test/runs",
			ProcDir:            "/test/proc",
			QueueDir:           "/test/queue",
			ServiceRegistryDir: "/test/service-registry",
		},
		UI: config.UI{
			LogEncodingCharset:    "iso-8859-1",
			NavbarColor:           "#123456",
			NavbarTitle:           "Test Dagu",
			MaxDashboardPageLimit: 250,
			DAGs: config.DAGsConfig{
				SortField: "status",
				SortOrder: "desc",
			},
		},
		Queues: config.Queues{Enabled: false},
		Worker: config.Worker{
			ID:            "test-worker-123",
			MaxActiveRuns: 200,
		},
		Scheduler: config.Scheduler{
			Port:                    9999,
			LockStaleThreshold:      30 * time.Second,
			LockRetryInterval:       5 * time.Second,
			ZombieDetectionInterval: 90 * time.Second,
		},
	}

	assert.Equal(t, expected, cfg)
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
  maxDashboardPageLimit: 50
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
tls:
  certFile: "/path/to/cert.pem"
  keyFile: "/path/to/key.pem"
scheduler:
  port: 7890
  lockStaleThreshold: 50s
  lockRetryInterval: 10s
  zombieDetectionInterval: 60s
`)

	utcLoc, _ := time.LoadLocation("UTC")

	expected := &config.Config{
		Global: config.Global{
			Debug:         true,
			LogFormat:     "json",
			TZ:            "UTC",
			TzOffsetInSec: 0,
			Location:      utcLoc,
			SkipExamples:  false,
			Peer:          config.Peer{Insecure: true}, // Default is true
			BaseEnv:       cfg.Global.BaseEnv,          // Dynamic, copy from actual
		},
		Server: config.Server{
			Host:        "0.0.0.0",
			Port:        9090,
			BasePath:    "/dagu",
			APIBasePath: "/api/v1",
			Headless:    true,
			Auth: config.Auth{
				Basic: config.AuthBasic{Username: "admin", Password: "secret"},
				Token: config.AuthToken{Value: "api-token"},
				OIDC: config.AuthOIDC{
					ClientId:     "test-client-id",
					ClientSecret: "test-client-secret",
					ClientUrl:    "http://localhost:8081",
					Issuer:       "https://accounts.example.com",
					Scopes:       []string{"openid", "profile", "email"},
					Whitelist:    []string{"user@example.com"},
				},
			},
			TLS: &config.TLSConfig{
				CertFile: "/path/to/cert.pem",
				KeyFile:  "/path/to/key.pem",
			},
			RemoteNodes: []config.RemoteNode{
				{Name: "node1", APIBaseURL: "http://node1.example.com/api"},
			},
			Permissions: map[config.Permission]bool{
				config.PermissionWriteDAGs: false,
				config.PermissionRunDAGs:   false,
			},
			LatestStatusToday: false,
			StrictValidation:  false,
		},
		Paths: config.PathsConfig{
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
		},
		UI: config.UI{
			LogEncodingCharset:    "utf-8",
			NavbarTitle:           "Test Dagu",
			MaxDashboardPageLimit: 50,
			DAGs:                  cfg.UI.DAGs, // Copy actual to avoid test pollution
		},
		Queues: config.Queues{Enabled: cfg.Queues.Enabled}, // Copy actual to avoid test pollution
		Worker: config.Worker{
			MaxActiveRuns: 100,
		},
		Scheduler: config.Scheduler{
			Port:                    7890,
			LockStaleThreshold:      50 * time.Second,
			LockRetryInterval:       10 * time.Second,
			ZombieDetectionInterval: 60 * time.Second,
		},
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
}

func TestLoad_EdgeCases_Errors(t *testing.T) {
	t.Run("InvalidTimezone", func(t *testing.T) {
		viper.Reset()
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		err := os.WriteFile(configFile, []byte(`tz: "Invalid/Timezone"`), 0600)
		require.NoError(t, err)

		_, err = config.Load(config.WithConfigFile(configFile))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load timezone")
	})

	t.Run("IncompleteTLS", func(t *testing.T) {
		viper.Reset()
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		err := os.WriteFile(configFile, []byte(`
tls:
  certFile: "/path/to/cert.pem"
  keyFile: ""
`), 0600)
		require.NoError(t, err)

		_, err = config.Load(config.WithConfigFile(configFile))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TLS configuration incomplete")
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
}

func TestLoad_LegacyEnv(t *testing.T) {
	cfg := loadWithEnv(t, "# empty", map[string]string{
		"DAGU__ADMIN_PORT":         "1234",
		"DAGU__ADMIN_HOST":         "0.0.0.0",
		"DAGU__ADMIN_NAVBAR_TITLE": "LegacyTitle",
		"DAGU__ADMIN_NAVBAR_COLOR": "#abc123",
		"DAGU__DATA":               "/legacy/data",
		"DAGU__SUSPEND_FLAGS_DIR":  "/legacy/suspend",
		"DAGU__ADMIN_LOGS_DIR":     "/legacy/adminlogs",
	})

	assert.Equal(t, 1234, cfg.Server.Port)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, "LegacyTitle", cfg.UI.NavbarTitle)
	assert.Equal(t, "#abc123", cfg.UI.NavbarColor)
	assert.Equal(t, "/legacy/data", cfg.Paths.DataDir)
	assert.Equal(t, "/legacy/suspend", cfg.Paths.SuspendFlagsDir)
	assert.Equal(t, "/legacy/adminlogs", cfg.Paths.AdminLogsDir)
}

func TestLoad_LoadLegacyFields(t *testing.T) {
	loader := &config.ConfigLoader{}

	t.Run("AllFieldsSet", func(t *testing.T) {
		def := config.Definition{
			BasicAuthUsername: "user",
			BasicAuthPassword: "pass",
			DAGsDir:           "/dags",
			NavbarTitle:       "Title",
		}

		cfg := config.Config{}
		loader.LoadLegacyFields(&cfg, def)

		assert.Equal(t, "user", cfg.Server.Auth.Basic.Username)
		assert.Equal(t, "pass", cfg.Server.Auth.Basic.Password)
		assert.Equal(t, "/dags", cfg.Paths.DAGsDir)
		assert.Equal(t, "Title", cfg.UI.NavbarTitle)
	})
}

// loadWithEnv loads config with environment variables set
func loadWithEnv(t *testing.T, yaml string, env map[string]string) *config.Config {
	t.Helper()
	viper.Reset()

	// Set environment variables
	for k, v := range env {
		original := os.Getenv(k)
		os.Setenv(k, v)
		t.Cleanup(func() {
			if original == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, original)
			}
		})
	}

	return loadFromYAML(t, yaml)
}

// loadFromYAML loads config from YAML string
func loadFromYAML(t *testing.T, yaml string) *config.Config {
	t.Helper()
	viper.Reset()

	configFile := filepath.Join(t.TempDir(), "config.yaml")

	err := os.WriteFile(configFile, []byte(yaml), 0600)
	require.NoError(t, err)

	cfg, err := config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
	return cfg
}
