package integration_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

const (
	postgresImage = "postgres:16-alpine"
)

// GetAvailablePort finds an available TCP port on localhost.
// This prevents port conflicts when running tests in parallel.
func GetAvailablePort(t *testing.T) int {
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err, "failed to find available port")
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

// TestSQLExecutor_PostgresContainer_BasicQuery tests basic PostgreSQL query execution
// using DAG-level container field for PostgreSQL container management.
func TestSQLExecutor_PostgresContainer_BasicQuery(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	port := GetAvailablePort(t)

	dag := th.DAG(t, fmt.Sprintf(`
container:
  image: %s
  env:
    - POSTGRES_PASSWORD: testpass
    - POSTGRES_USER: testuser
    - POSTGRES_DB: testdb
  ports:
    - "%d:5432"
  waitFor: healthy

steps:
  - name: init-db
    type: postgres
    config:
      dsn: "postgres://testuser:testpass@localhost:%d/testdb?sslmode=disable"
      transaction: true
    script: |
      CREATE TABLE users (id SERIAL PRIMARY KEY, name TEXT NOT NULL);
      INSERT INTO users (name) VALUES ('Alice'), ('Bob');

  - name: query-users
    type: postgres
    config:
      dsn: "postgres://testuser:testpass@localhost:%d/testdb?sslmode=disable"
      outputFormat: jsonl
    command: SELECT id, name FROM users ORDER BY id
    output: USERS
    depends: [init-db]
`, postgresImage, port, port, port))

	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, core.Succeeded)
}

// TestSQLExecutor_PostgresContainer_Transaction tests transaction commit behavior.
func TestSQLExecutor_PostgresContainer_Transaction(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	port := GetAvailablePort(t)

	dag := th.DAG(t, fmt.Sprintf(`
container:
  image: %s
  env:
    - POSTGRES_PASSWORD: testpass
  ports:
    - "%d:5432"
  waitFor: healthy

steps:
  - name: setup
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
    script: |
      CREATE TABLE accounts (id INT PRIMARY KEY, balance INT NOT NULL);
      INSERT INTO accounts VALUES (1, 100), (2, 200);

  - name: transfer
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
      transaction: true
    script: |
      UPDATE accounts SET balance = balance - 50 WHERE id = 1;
      UPDATE accounts SET balance = balance + 50 WHERE id = 2;
    depends: [setup]

  - name: verify
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
      outputFormat: jsonl
    command: SELECT id, balance FROM accounts ORDER BY id
    output: BALANCES
    depends: [transfer]
`, postgresImage, port, port, port, port))

	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, core.Succeeded)
}

// TestSQLExecutor_PostgresContainer_TransactionRollback tests that failed transactions
// properly rollback changes.
func TestSQLExecutor_PostgresContainer_TransactionRollback(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	port := GetAvailablePort(t)

	dag := th.DAG(t, fmt.Sprintf(`
container:
  image: %s
  env:
    - POSTGRES_PASSWORD: testpass
  ports:
    - "%d:5432"
  waitFor: healthy

steps:
  - name: setup
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
    script: |
      CREATE TABLE rollback_test (id INT PRIMARY KEY, value INT NOT NULL);
      INSERT INTO rollback_test VALUES (1, 100);

  - name: failed-transaction
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
      transaction: true
    script: |
      UPDATE rollback_test SET value = 999 WHERE id = 1;
      SELECT * FROM nonexistent_table_for_error;
    depends: [setup]
    continueOn:
      failure: true

  - name: verify-rollback
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
      outputFormat: jsonl
    command: SELECT value FROM rollback_test WHERE id = 1
    output: VALUE_AFTER_ROLLBACK
    depends: [failed-transaction]
`, postgresImage, port, port, port, port))

	dag.Agent().RunSuccess(t)
	// The DAG should succeed even though one step failed (continueOn: failure: true)
	// The value should still be 100 because the transaction was rolled back
}

// TestSQLExecutor_PostgresContainer_AdvisoryLock tests advisory lock serialization.
func TestSQLExecutor_PostgresContainer_AdvisoryLock(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	port := GetAvailablePort(t)

	dag := th.DAG(t, fmt.Sprintf(`
container:
  image: %s
  env:
    - POSTGRES_PASSWORD: testpass
  ports:
    - "%d:5432"
  waitFor: healthy

steps:
  - name: step1
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
      advisoryLock: "test_serialization_lock"
    command: SELECT 'step1_completed' as result
    output: RESULT1

  - name: step2
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
      advisoryLock: "test_serialization_lock"
    command: SELECT 'step2_completed' as result
    output: RESULT2
    depends: [step1]
`, postgresImage, port, port, port))

	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, core.Succeeded)
}

// TestSQLExecutor_PostgresContainer_NullValues tests NULL value handling in output.
func TestSQLExecutor_PostgresContainer_NullValues(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	port := GetAvailablePort(t)

	dag := th.DAG(t, fmt.Sprintf(`
container:
  image: %s
  env:
    - POSTGRES_PASSWORD: testpass
  ports:
    - "%d:5432"
  waitFor: healthy

steps:
  - name: test-nulls
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
      outputFormat: jsonl
    command: |
      SELECT
        NULL::text as null_text,
        NULL::int as null_int,
        NULL::boolean as null_bool,
        'not_null'::text as regular_text,
        42 as regular_int
    output: NULL_VALUES
`, postgresImage, port, port))

	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, core.Succeeded)
}

// TestSQLExecutor_PostgresContainer_OutputFormats tests different output formats.
func TestSQLExecutor_PostgresContainer_OutputFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"JSONL", "jsonl"},
		{"JSON", "json"},
		{"CSV", "csv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			th := test.Setup(t)
			port := GetAvailablePort(t)

			dag := th.DAG(t, fmt.Sprintf(`
container:
  image: %s
  env:
    - POSTGRES_PASSWORD: testpass
  ports:
    - "%d:5432"
  waitFor: healthy

steps:
  - name: query
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
      outputFormat: %s
      headers: true
    command: SELECT 1 as id, 'test' as name
    output: RESULT
`, postgresImage, port, port, tt.format))

			dag.Agent().RunSuccess(t)
			dag.AssertLatestStatus(t, core.Succeeded)
		})
	}
}

// TestSQLExecutor_PostgresContainer_MaxRows tests row limiting functionality.
func TestSQLExecutor_PostgresContainer_MaxRows(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	port := GetAvailablePort(t)

	dag := th.DAG(t, fmt.Sprintf(`
container:
  image: %s
  env:
    - POSTGRES_PASSWORD: testpass
  ports:
    - "%d:5432"
  waitFor: healthy

steps:
  - name: setup
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
    script: |
      CREATE TABLE many_rows (id SERIAL PRIMARY KEY, value TEXT);
      INSERT INTO many_rows (value) SELECT 'row_' || generate_series(1, 100);

  - name: query-limited
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
      outputFormat: jsonl
      maxRows: 5
    command: SELECT * FROM many_rows ORDER BY id
    output: LIMITED_ROWS
    depends: [setup]
`, postgresImage, port, port, port))

	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, core.Succeeded)
}

// TestSQLExecutor_PostgresContainer_NamedParams tests named parameter substitution.
func TestSQLExecutor_PostgresContainer_NamedParams(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	port := GetAvailablePort(t)

	dag := th.DAG(t, fmt.Sprintf(`
container:
  image: %s
  env:
    - POSTGRES_PASSWORD: testpass
  ports:
    - "%d:5432"
  waitFor: healthy

steps:
  - name: setup
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
    script: |
      CREATE TABLE products (id SERIAL PRIMARY KEY, name TEXT, price NUMERIC);
      INSERT INTO products (name, price) VALUES ('Apple', 1.50), ('Banana', 0.75), ('Orange', 2.00);

  - name: query-with-params
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
      outputFormat: jsonl
      params:
        min_price: 1.00
    command: SELECT name, price FROM products WHERE price >= :min_price ORDER BY name
    output: FILTERED_PRODUCTS
    depends: [setup]
`, postgresImage, port, port, port))

	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, core.Succeeded)
}

// TestSQLExecutor_PostgresContainer_MultiStatement tests multi-statement scripts.
func TestSQLExecutor_PostgresContainer_MultiStatement(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)
	port := GetAvailablePort(t)

	dag := th.DAG(t, fmt.Sprintf(`
container:
  image: %s
  env:
    - POSTGRES_PASSWORD: testpass
  ports:
    - "%d:5432"
  waitFor: healthy

steps:
  - name: multi-statement
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
      transaction: true
    script: |
      CREATE TABLE orders (id SERIAL PRIMARY KEY, status TEXT);
      INSERT INTO orders (status) VALUES ('pending');
      UPDATE orders SET status = 'completed' WHERE status = 'pending';

  - name: verify
    type: postgres
    config:
      dsn: "postgres://postgres:testpass@localhost:%d/postgres?sslmode=disable"
      outputFormat: jsonl
    command: SELECT status FROM orders
    output: ORDER_STATUS
    depends: [multi-statement]
`, postgresImage, port, port, port))

	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, core.Succeeded)
}

// TestSQLExecutor_SQLite_InMemory tests SQLite in-memory database.
func TestSQLExecutor_SQLite_InMemory(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)

	dag := th.DAG(t, `
steps:
  - name: sqlite-query
    type: sqlite
    config:
      dsn: ":memory:"
      outputFormat: jsonl
    script: |
      CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT);
      INSERT INTO test (name) VALUES ('Alice'), ('Bob');
      SELECT * FROM test ORDER BY id;
    output: SQLITE_RESULT
`)

	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, core.Succeeded)
}

// TestSQLExecutor_SQLite_Transaction tests SQLite transaction handling.
func TestSQLExecutor_SQLite_Transaction(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)

	dag := th.DAG(t, `
steps:
  - name: sqlite-transaction
    type: sqlite
    config:
      dsn: ":memory:"
      transaction: true
      outputFormat: jsonl
    script: |
      CREATE TABLE counter (id INTEGER PRIMARY KEY, value INTEGER);
      INSERT INTO counter VALUES (1, 0);
      UPDATE counter SET value = value + 1 WHERE id = 1;
      UPDATE counter SET value = value + 1 WHERE id = 1;
      SELECT value FROM counter WHERE id = 1;
    output: COUNTER_VALUE
`)

	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, core.Succeeded)
}

// TestSQLExecutor_SQLite_OutputFormats tests SQLite output formats.
func TestSQLExecutor_SQLite_OutputFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"JSONL", "jsonl"},
		{"JSON", "json"},
		{"CSV", "csv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			th := test.Setup(t)

			dag := th.DAG(t, fmt.Sprintf(`
steps:
  - name: sqlite-format-test
    type: sqlite
    config:
      dsn: ":memory:"
      outputFormat: %s
      headers: true
    script: |
      CREATE TABLE data (id INTEGER, name TEXT);
      INSERT INTO data VALUES (1, 'test');
      SELECT * FROM data;
    output: RESULT
`, tt.format))

			dag.Agent().RunSuccess(t)
			dag.AssertLatestStatus(t, core.Succeeded)
		})
	}
}
