package settings

import (
	"fmt"
	"os"
	"path"

	"github.com/yohamta/dagu/internal/utils"
)

var ErrConfigNotFound = fmt.Errorf("config not found")

var cache map[string]string = nil

const (
	CONFIG__DATA_DIR           = "DAGU__DATA"
	CONFIG__LOGS_DIR           = "DAGU__LOGS"
	CONFIG__ADMIN_PORT         = "DAGU__ADMIN_PORT"
	CONFIG__ADMIN_NAVBAR_COLOR = "DAGU__ADMIN_NAVBAR_COLOR"
	CONFIG__ADMIN_NAVBAR_TITLE = "DAGU__ADMIN_NAVBAR_TITLE"
)

func MustGet(name string) string {
	val, err := Get(name)
	if err != nil {
		panic(fmt.Errorf("failed to get %s : %w", name, err))
	}
	return val
}

func init() {
	load()
}

func Get(name string) (string, error) {
	if val, ok := cache[name]; ok {
		return val, nil
	}
	return "", ErrConfigNotFound
}

func load() {
	dir := utils.MustGetUserHomeDir()

	cache = map[string]string{}
	cache[CONFIG__DATA_DIR] = config(
		CONFIG__DATA_DIR,
		path.Join(dir, "/.dagu/data"))
	cache[CONFIG__LOGS_DIR] = config(CONFIG__LOGS_DIR,
		path.Join(dir, "/.dagu/logs"))
	cache[CONFIG__ADMIN_PORT] = config(CONFIG__ADMIN_PORT, "8080")
	cache[CONFIG__ADMIN_NAVBAR_COLOR] = config(CONFIG__ADMIN_NAVBAR_COLOR, "")
	cache[CONFIG__ADMIN_NAVBAR_TITLE] = config(CONFIG__ADMIN_NAVBAR_TITLE, "Dagu admin")
}

func InitTest(dir string) {
	os.Setenv("HOME", dir)
	load()
}

func config(env, def string) string {
	val := os.ExpandEnv(fmt.Sprintf("${%s}", env))
	if val == "" {
		return def
	}
	return val
}
