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

package scheduler

import (
	"os"
	"testing"

	"github.com/daguflow/dagu/internal/util"
	"go.uber.org/goleak"
)

var testHomeDir string

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
	tempDir := util.MustTempDir("runner_test")
	err := os.Setenv("HOME", tempDir)
	if err != nil {
		panic(err)
	}
	testHomeDir = tempDir
	code := m.Run()
	_ = os.RemoveAll(tempDir)
	os.Exit(code)
}
