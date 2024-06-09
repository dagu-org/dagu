package dag

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
