package settings

import (
	"fmt"
	"os"
	"path"

	"github.com/yohamta/dagu/internal/utils"
)

var (
	ErrSettingNotFound = fmt.Errorf("setting not found")
)

const (
	SETTING__DATA_DIR           = "DAGU__DATA"
	SETTING__LOGS_DIR           = "DAGU__LOGS"
	SETTING__SUSPEND_FLAGS_DIR  = "DAGU__SUSPEND_FLAGS_DIR"
	SETTING__ADMIN_PORT         = "DAGU__ADMIN_PORT"
	SETTING__ADMIN_NAVBAR_COLOR = "DAGU__ADMIN_NAVBAR_COLOR"
	SETTING__ADMIN_NAVBAR_TITLE = "DAGU__ADMIN_NAVBAR_TITLE"
	SETTING__ADMIN_LOGS_DIR     = "DAGU__ADMIN_LOGS_DIR"
)

// MustGet returns the value of the setting or
// panics if the setting is not found.
func MustGet(name string) string {
	val, err := Get(name)
	if err != nil {
		panic(fmt.Errorf("failed to get %s : %w", name, err))
	}
	return val
}

// Get returns the value of the setting or ErrSettingNotFound
func Get(name string) (string, error) {
	if val, ok := cache[name]; ok {
		return val, nil
	}
	return "", ErrSettingNotFound
}

// ChangeHomeDir changes the home directory and reloads
// the settings.
func ChangeHomeDir(homeDir string) {
	os.Setenv("HOME", homeDir)
	load()
}

var cache map[string]string = nil

func init() {
	load()
}

func load() {
	homeDir := utils.MustGetUserHomeDir()

	cache = map[string]string{}
	cache[SETTING__DATA_DIR] = readEnv(
		SETTING__DATA_DIR,
		path.Join(homeDir, "/.dagu/data"))
	cache[SETTING__LOGS_DIR] = readEnv(SETTING__LOGS_DIR,
		path.Join(homeDir, "/.dagu/logs"))
	cache[SETTING__SUSPEND_FLAGS_DIR] = readEnv(SETTING__SUSPEND_FLAGS_DIR,
		path.Join(homeDir, "/.dagu/suspend"))

	cache[SETTING__ADMIN_PORT] = readEnv(SETTING__ADMIN_PORT, "8080")
	cache[SETTING__ADMIN_NAVBAR_COLOR] = readEnv(SETTING__ADMIN_NAVBAR_COLOR, "")
	cache[SETTING__ADMIN_NAVBAR_TITLE] = readEnv(SETTING__ADMIN_NAVBAR_TITLE, "Dagu admin")
	cache[SETTING__ADMIN_LOGS_DIR] = readEnv(SETTING__ADMIN_LOGS_DIR,
		path.Join(homeDir, "/.dagu/logs/admin"))
}

func readEnv(env, def string) string {
	return utils.StringWithFallback(
		os.ExpandEnv(fmt.Sprintf("${%s}", env)),
		def,
	)
}
