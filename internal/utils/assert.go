package utils

import (
	"regexp"
	"testing"
)

func AssertPattern(t *testing.T, name string, want string, actual string) {
	re := regexp.MustCompile(want)

	if !re.Match([]byte(actual)) {
		t.Fatalf("%s should match %s, was %s", name, want, actual)
	}
}
