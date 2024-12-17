// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

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
