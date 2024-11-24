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

package model

import (
	"encoding/json"
	"testing"

	"github.com/dagu-org/dagu/internal/dag"
)

func TestJsonMarshal(t *testing.T) {
	step := dag.Step{
		OutputVariables: &dag.SyncMap{},
	}
	step.OutputVariables.Store("A", "B")
	rawJSON, err := json.Marshal(step)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(rawJSON))
}
