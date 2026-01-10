package sql

// Export internal functions for testing.
// This file is only compiled during testing (due to _test.go suffix).

// SplitStatements exports splitStatements for testing.
var SplitStatements = splitStatements

// IsSelectQuery exports isSelectQuery for testing.
var IsSelectQuery = isSelectQuery
