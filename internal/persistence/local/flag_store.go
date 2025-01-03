package local

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/local/storage"
)

type flagStoreImpl struct {
	storage *storage.Storage
}

func NewFlagStore(s *storage.Storage) persistence.FlagStore {
	return &flagStoreImpl{
		storage: s,
	}
}

func (f flagStoreImpl) ToggleSuspend(id string, suspend bool) error {
	if suspend {
		return f.storage.Create(fileName(id))
	} else if f.IsSuspended(id) {
		return f.storage.Delete(fileName(id))
	}
	return nil
}

func (f flagStoreImpl) IsSuspended(id string) bool {
	return f.storage.Exists(fileName(id))
}

func fileName(id string) string {
	return fmt.Sprintf("%s.suspend", normalizeFilename(id, "-"))
}

// https://github.com/sindresorhus/filename-reserved-regex/blob/master/index.js
var (
	filenameReservedRegex = regexp.MustCompile(
		`[<>:"/\\|?*\x00-\x1F]`,
	)
	filenameReservedWindowsNamesRegex = regexp.MustCompile(
		`(?i)^(con|prn|aux|nul|com[0-9]|lpt[0-9])$`,
	)
)

func normalizeFilename(str, replacement string) string {
	s := filenameReservedRegex.ReplaceAllString(str, replacement)
	s = filenameReservedWindowsNamesRegex.ReplaceAllString(s, replacement)
	return strings.ReplaceAll(s, " ", replacement)
}
