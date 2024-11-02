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

package dag

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCondition_Eval(t *testing.T) {
	tests := []struct {
		name      string
		condition []Condition
		wantErr   bool
	}{
		{
			name:      "CommandSubstitution",
			condition: []Condition{{Condition: "`echo 1`", Expected: "1"}},
		},
		{
			name:      "EnvVar",
			condition: []Condition{{Condition: "${TEST_CONDITION}", Expected: "100"}},
		},
		{
			name: "MultipleCond",
			condition: []Condition{
				{
					Condition: "`echo 1`",
					Expected:  "1",
				},
				{
					Condition: "`echo 100`",
					Expected:  "100",
				},
			},
		},
		{
			name: "MultipleCondOneMet",
			condition: []Condition{
				{
					Condition: "`echo 1`",
					Expected:  "1",
				},
				{
					Condition: "`echo 100`",
					Expected:  "1",
				},
			},
			wantErr: true,
		},
		{
			name: "InvalidCond",
			condition: []Condition{
				{
					Condition: "`invalid`",
				},
			},
			wantErr: true,
		},
		{
			name: "ComplexCondition",
			condition: []Condition{
				{
					Condition: "`(or (eq $1 foo) (eq $1 bar))`",
					Expected:  "true",
				},
			},
		},
		{
			name: "ComplexCondition1",
			condition: []Condition{
				{
					Condition: "(and (eq $1 foo) (eq $1 bar))",
					Expected:  "true",
				},
			},
			wantErr: true,
		},
	}

	// Set environment variable for testing
	_ = os.Setenv("TEST_CONDITION", "100")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EvalConditions(tt.condition)
			require.Equal(t, tt.wantErr, err != nil)
		})
	}
}
