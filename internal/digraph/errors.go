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

package digraph

import "strings"

// errorList is just a list of errors.
// It is used to collect multiple errors in building a DAG.
type errorList []error

// Add adds an error to the list.
func (e *errorList) Add(err error) {
	if err != nil {
		*e = append(*e, err)
	}
}

// Error implements the error interface.
// It returns a string with all the errors separated by a semicolon.
func (e *errorList) Error() string {
	errStrings := make([]string, len(*e))
	for i, err := range *e {
		errStrings[i] = err.Error()
	}
	return strings.Join(errStrings, "; ")
}
