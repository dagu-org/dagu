// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
