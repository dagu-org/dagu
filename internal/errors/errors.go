package errors

import "strings"

type ErrorList struct {
	errors []error
}

func (e *ErrorList) Add(err error) {
	if err != nil {
		e.errors = append(e.errors, err)
	}
}

func (e *ErrorList) Error() string {
	errStrings := make([]string, len(e.errors))
	for i, err := range e.errors {
		errStrings[i] = err.Error()
	}
	return strings.Join(errStrings, "; ")
}

func (e *ErrorList) HasErrors() bool {
	return len(e.errors) > 0
}
