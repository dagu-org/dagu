package storage

import (
	// nolint: gosec
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
)

type Address struct {
	dagName       string
	prefix        string
	executionsDir string
	globPattern   string
	rootDAG       *digraph.RootDAG
}

type AddressOption func(*Address)

func WithRootDAG(rootDAG *digraph.RootDAG) AddressOption {
	return func(a *Address) {
		a.rootDAG = rootDAG
	}
}

func NewAddress(baseDir, dagName string, opts ...AddressOption) Address {
	ext := filepath.Ext(dagName)
	a := Address{dagName: dagName}

	for _, opt := range opts {
		opt(&a)
	}

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
	a.executionsDir = filepath.Join(baseDir, a.prefix, "executions")
	a.globPattern = filepath.Join(a.executionsDir, "2*", "*", "*", "exec_*", "status"+dataFileExtension)

	return a
}

func (a Address) GlobPatternWithRequestID(requestID string) string {
	return filepath.Join(a.executionsDir, "2*", "*", "*", "exec_*"+requestID+"*", "status"+dataFileExtension)
}

func (a Address) FilePath(timestamp TimeInUTC, requestID string) string {
	year := timestamp.Format("2006")
	month := timestamp.Format("01")
	date := timestamp.Format("02")
	ts := timestamp.Format(dateTimeFormatUTC)
	dirName := "exec_" + ts + "_" + requestID
	return filepath.Join(a.executionsDir, year, month, date, dirName, "status"+dataFileExtension)
}

func (a Address) Exists() bool {
	_, err := os.Stat(a.executionsDir)
	return !os.IsNotExist(err)
}

func (a Address) Create() error {
	if err := os.MkdirAll(a.executionsDir, 0755); err != nil {
		return fmt.Errorf("%w: %s : %s", ErrCreateNewDirectory, a.executionsDir, err)
	}
	return nil
}

func (a Address) IsEmpty() bool {
	files, _ := os.ReadDir(a.executionsDir)
	return len(files) == 0
}

func (a Address) Remove() error {
	if err := os.RemoveAll(a.executionsDir); err != nil {
		return fmt.Errorf("%w: %s : %s", ErrRemoveDirectory, a.executionsDir, err)
	}
	return nil
}
