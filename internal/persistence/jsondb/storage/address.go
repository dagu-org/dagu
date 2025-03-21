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
	dagName     string
	prefix      string
	path        string
	globPattern string
	rootDAG     *digraph.RootDAG
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
	a.path = filepath.Join(baseDir, a.prefix)
	a.globPattern = filepath.Join(a.path, "2*", "*", "*", "*", "*"+dataFileExtension)

	return a
}

func (a Address) GlobPatternWithRequestID(requestID string) string {
	return filepath.Join(a.path, "2*", "*", "*", "2*"+requestID+"*", "status"+dataFileExtension)
}

func (a Address) FilePath(timestamp TimeInUTC, requestID string) string {
	year := timestamp.Format("2006")
	month := timestamp.Format("01")
	date := timestamp.Format("02")
	ts := timestamp.Format(dateTimeFormatUTC)
	dirName := ts + "_" + requestID
	return filepath.Join(a.path, year, month, date, dirName, "status"+dataFileExtension)
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
