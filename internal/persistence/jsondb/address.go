package jsondb

import (
	"crypto/md5" // nolint: gosec
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/fileutil"
)

type Address struct {
	dagNameOrPath string
	prefix        string
	path          string
}

func NewAddress(baseDir string, dagNameOrPath string) Address {
	ext := filepath.Ext(dagNameOrPath)
	addr := Address{dagNameOrPath: dagNameOrPath}

	switch {
	case ext == "":
		// No extension
		addr.prefix = filepath.Base(dagNameOrPath)

	case fileutil.IsYAMLFile(dagNameOrPath):
		// Remove .yaml or .yml extension
		addr.prefix = strings.TrimSuffix(filepath.Base(dagNameOrPath), ext)

	default:
		// Use the base name (if it's a path or just a name)
		addr.prefix = filepath.Base(dagNameOrPath)
		// TODO: Convert it to a safe name
	}

	if dagNameOrPath != addr.prefix {
		// Legacy behavior: Add a hash postfix to the directory name to avoid conflicts.
		// nolint: gosec
		h := md5.New()
		_, _ = h.Write([]byte(addr.dagNameOrPath))
		v := hex.EncodeToString(h.Sum(nil))
		addr.path = filepath.Join(baseDir, addr.prefix+"-"+v)
	}

	addr.path = filepath.Join(baseDir, addr.prefix)
	return addr
}

func (a Address) Exists() bool {
	_, err := os.Stat(a.path)
	return !os.IsNotExist(err)
}

func (a Address) Create() error {
	if err := os.MkdirAll(a.path, 0755); err != nil {
		return fmt.Errorf("%w: %s : %s", ErrCreateNewDirectory, a.path, err)
	}
	return nil
}

func (a Address) IsEmpty() bool {
	files, _ := os.ReadDir(a.path)
	return len(files) == 0
}

func (a Address) Remove() error {
	if err := os.RemoveAll(a.path); err != nil {
		return fmt.Errorf("%w: %s : %s", ErrRemoveDirectory, a.path, err)
	}
	return nil
}
