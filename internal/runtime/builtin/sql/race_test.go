package sql_test

import (
	"bytes"
	"context"
	"os"
	"sync"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime/executor"
	"github.com/stretchr/testify/require"

	sqlexec "github.com/dagu-org/dagu/internal/runtime/builtin/sql"
	// Import drivers for testing
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/sql/drivers/sqlite"
)

// TestRace_ConcurrentExecutors tests that multiple executors can run concurrently
// without data races. Run with: go test -race ./internal/runtime/builtin/sql/...
func TestRace_ConcurrentExecutors(t *testing.T) {
	ctx := context.Background()
	numExecutors := 10

	var wg sync.WaitGroup

	for i := 0; i < numExecutors; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			step := core.Step{
				Name: "test-concurrent",
				ExecutorConfig: core.ExecutorConfig{
					Type: "sqlite",
					Config: map[string]any{
						"dsn": ":memory:",
					},
				},
				Script: `
					CREATE TABLE test (id INTEGER PRIMARY KEY, value INTEGER);
					INSERT INTO test (value) VALUES (1), (2), (3);
					SELECT * FROM test;
				`,
			}

			exec, err := executor.NewExecutor(ctx, step)
			require.NoError(t, err)

			var stdout bytes.Buffer
			exec.SetStdout(&stdout)

			err = exec.Run(ctx)
			require.NoError(t, err)
		}(i)
	}

	wg.Wait()
}

// TestRace_ConnectionManagerConcurrent tests concurrent acquire/release operations
// on ConnectionManager for race conditions.
func TestRace_ConnectionManagerConcurrent(t *testing.T) {
	ctx := context.Background()

	cfg, err := sqlexec.ParseConfig(ctx, map[string]any{
		"dsn": ":memory:",
	})
	require.NoError(t, err)

	driver, ok := sqlexec.GetDriver("sqlite")
	require.True(t, ok)

	cm, err := sqlexec.NewConnectionManager(ctx, driver, cfg)
	require.NoError(t, err)

	numGoroutines := 20
	var wg sync.WaitGroup

	// Test concurrent acquire/release/DB access
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			cm.Acquire()
			defer cm.Release()

			// Concurrent DB access
			_, err := cm.DB().ExecContext(ctx, "SELECT 1")
			if err != nil {
				// Ignore errors - we're testing for data races, not functionality
				return
			}
		}()
	}

	wg.Wait()

	// Final release
	_ = cm.Release()
}

// TestRace_DriverRegistryConcurrent tests concurrent access to the driver registry.
func TestRace_DriverRegistryConcurrent(t *testing.T) {
	numGoroutines := 50
	var wg sync.WaitGroup

	// Test concurrent GetDriver calls
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Concurrent GetDriver calls
			driver, ok := sqlexec.GetDriver("sqlite")
			if ok && driver != nil {
				_ = driver.Name()
				_ = driver.PlaceholderFormat()
				_ = driver.SupportsAdvisoryLock()
			}
		}()
	}

	wg.Wait()
}

// TestRace_TransactionConcurrent tests concurrent transaction operations.
func TestRace_TransactionConcurrent(t *testing.T) {
	ctx := context.Background()

	cfg, err := sqlexec.ParseConfig(ctx, map[string]any{
		"dsn": ":memory:",
	})
	require.NoError(t, err)

	driver, ok := sqlexec.GetDriver("sqlite")
	require.True(t, ok)

	cm, err := sqlexec.NewConnectionManager(ctx, driver, cfg)
	require.NoError(t, err)
	defer cm.Close()

	db := cm.DB()

	// Create table
	_, err = db.ExecContext(ctx, "CREATE TABLE concurrent_test (id INTEGER PRIMARY KEY, value INTEGER)")
	require.NoError(t, err)

	numGoroutines := 10
	var wg sync.WaitGroup

	// Test concurrent transaction creation and usage
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			tx, err := sqlexec.BeginTransaction(ctx, db, "")
			if err != nil {
				// Transaction creation may fail due to database contention
				return
			}

			// Execute within transaction
			_, err = tx.Tx().ExecContext(ctx, "INSERT INTO concurrent_test (value) VALUES (?)", idx)
			if err != nil {
				_ = tx.Rollback()
				return
			}

			// Randomly commit or rollback
			if idx%2 == 0 {
				_ = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
		}(i)
	}

	wg.Wait()
}

// TestRace_ResultWriter tests concurrent result writer operations.
func TestRace_ResultWriter(t *testing.T) {
	numGoroutines := 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var buf bytes.Buffer
			writer := sqlexec.NewResultWriter(&buf, "jsonl", "null", false)

			_ = writer.WriteHeader([]string{"id", "name", "value"})

			for j := 0; j < 100; j++ {
				_ = writer.WriteRow([]any{j, "test", 3.14})
			}

			_ = writer.Close()
		}()
	}

	wg.Wait()
}

// TestRace_ParamConversion tests concurrent parameter conversion.
func TestRace_ParamConversion(t *testing.T) {
	numGoroutines := 20
	var wg sync.WaitGroup

	queries := []string{
		"SELECT * FROM users WHERE id = :id",
		"SELECT * FROM users WHERE name = :name AND status = :status",
		"SELECT * FROM users WHERE id = :id OR parent_id = :id",
	}

	params := []map[string]any{
		{"id": 123},
		{"name": "Alice", "status": "active"},
		{"id": 456},
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			query := queries[idx%len(queries)]
			param := params[idx%len(params)]

			// Test named to positional conversion
			_, _, _ = sqlexec.ConvertNamedToPositional(query, param, "$")
			_, _, _ = sqlexec.ConvertNamedToPositional(query, param, "?")
		}(i)
	}

	wg.Wait()
}

// TestRace_ConfigParsing tests concurrent config parsing.
func TestRace_ConfigParsing(t *testing.T) {
	ctx := context.Background()
	numGoroutines := 20
	var wg sync.WaitGroup

	configs := []map[string]any{
		{"dsn": ":memory:", "outputFormat": "jsonl"},
		{"dsn": ":memory:", "outputFormat": "json"},
		{"dsn": ":memory:", "outputFormat": "csv", "headers": true},
		{"dsn": ":memory:", "transaction": true, "isolationLevel": "serializable"},
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			cfg := configs[idx%len(configs)]
			_, _ = sqlexec.ParseConfig(ctx, cfg)
		}(i)
	}

	wg.Wait()
}

// TestRace_ExecutorKill tests calling Kill() concurrently with Run().
func TestRace_ExecutorKill(t *testing.T) {
	ctx := context.Background()
	numIterations := 5

	for i := 0; i < numIterations; i++ {
		step := core.Step{
			Name: "test-kill-race",
			ExecutorConfig: core.ExecutorConfig{
				Type: "sqlite",
				Config: map[string]any{
					"dsn": ":memory:",
				},
			},
			Script: `
				CREATE TABLE test (id INTEGER);
				INSERT INTO test VALUES (1), (2), (3), (4), (5);
				SELECT * FROM test;
			`,
		}

		exec, err := executor.NewExecutor(ctx, step)
		require.NoError(t, err)

		var stdout bytes.Buffer
		exec.SetStdout(&stdout)

		var wg sync.WaitGroup

		// Start Run() in goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Run may succeed or be killed - either is fine
			_ = exec.Run(ctx)
		}()

		// Kill() from another goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = exec.Kill(os.Interrupt)
		}()

		wg.Wait()
	}
}
