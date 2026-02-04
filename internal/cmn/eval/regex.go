package eval

import "regexp"

// reVarSubstitution matches $VAR, ${VAR}, '$VAR', '${VAR}' patterns for variable substitution.
// Group 1: ${...} content, Group 2: $VAR content (without braces)
var reVarSubstitution = regexp.MustCompile(`[']{0,1}\$\{([^}]+)\}[']{0,1}|[']{0,1}\$([a-zA-Z0-9_][a-zA-Z0-9_]*)[']{0,1}`)

// reQuotedJSONRef matches quoted JSON references like "${FOO.bar}" and simple variables like "${VAR}"
var reQuotedJSONRef = regexp.MustCompile(`"\$\{([A-Za-z0-9_]\w*(?:\.[^}]+)?)\}"`)

// reJSONPathRef matches patterns like ${FOO.bar.baz} or $FOO.bar for JSON path expansion
var reJSONPathRef = regexp.MustCompile(`\$\{([A-Za-z0-9_]\w*)(\.[^}]+)\}|\$([A-Za-z0-9_]\w*)(\.[^\s]+)`)
