package storage

import (
	// nolint: gosec
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/fileutil"
)

type Address struct {
	dagName     string
	prefix      string
	path        string
	globPattern string
}

func NewAddress(baseDir string, dagName string) Address {
	ext := filepath.Ext(dagName)
	a := Address{dagName: dagName}

	base := filepath.Base(dagName)
	if fileutil.IsYAMLFile(dagName) {
		// Remove .yaml or .yml extension
		base = strings.TrimSuffix(base, ext)
	}

	n := fileutil.SafeName(base)
	if n != base {
		hash := sha256.Sum256([]byte(dagName))
		hashLength := 4 // 4 characters of the hash should be enough
		n = n + "-" + hex.EncodeToString(hash[:])[0:hashLength]
	}

	a.prefix = n
	a.path = filepath.Join(baseDir, a.prefix)
	a.globPattern = path.Join(a.path, "20*", "*"+dataFileExtension)

	return a
}

func (a Address) GlobPatternWithRequestID(requestID string) string {
	return path.Join(a.path, "20*"+requestID+"*", "status"+dataFileExtension)
}

func (a Address) FilePath(timestamp TimeInUTC, requestID string) string {
	ts := timestamp.Format(dateTimeFormatUTC)
	dir := ts + "_" + requestID
	return path.Join(a.path, dir, "status"+dataFileExtension)
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
