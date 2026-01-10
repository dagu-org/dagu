package sql

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"time"
)

// ImportMetrics tracks import operation statistics.
type ImportMetrics struct {
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at"`
	DurationMs   int64     `json:"duration_ms"`
	RowsRead     int64     `json:"rows_read"`
	RowsImported int64     `json:"rows_imported"`
	RowsSkipped  int64     `json:"rows_skipped"`
	BatchCount   int       `json:"batch_count"`
	Status       string    `json:"status"`
	Error        string    `json:"error,omitempty"`
}

// Importer handles data import from files to database tables.
type Importer struct {
	db      *sql.DB
	tx      *sql.Tx
	driver  Driver
	cfg     *ImportConfig
	metrics ImportMetrics
}

// NewImporter creates a new Importer instance.
func NewImporter(db *sql.DB, tx *sql.Tx, driver Driver, cfg *ImportConfig) *Importer {
	return &Importer{
		db:     db,
		tx:     tx,
		driver: driver,
		cfg:    cfg,
	}
}

// Import executes the import operation.
func (i *Importer) Import(ctx context.Context) (*ImportMetrics, error) {
	i.metrics.StartedAt = time.Now()
	i.metrics.Status = "running"

	err := i.doImport(ctx)

	i.metrics.FinishedAt = time.Now()
	i.metrics.DurationMs = i.metrics.FinishedAt.Sub(i.metrics.StartedAt).Milliseconds()

	if err != nil {
		i.metrics.Status = "failed"
		i.metrics.Error = err.Error()
		return &i.metrics, err
	}

	i.metrics.Status = "completed"
	return &i.metrics, nil
}

func (i *Importer) doImport(ctx context.Context) error {
	// Open input file
	file, err := os.Open(i.cfg.InputFile)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	// Create input reader
	opts := i.buildInputOptions()
	reader, err := NewInputReader(file, i.cfg.Format, opts)
	if err != nil {
		return fmt.Errorf("failed to create input reader: %w", err)
	}
	defer reader.Close()

	// Read header to get columns
	columns, err := i.resolveColumns(reader)
	if err != nil {
		return err
	}

	if len(columns) == 0 {
		return fmt.Errorf("no columns found for import")
	}

	// Process rows in batches
	return i.processBatches(ctx, reader, columns)
}

func (i *Importer) buildInputOptions() InputOptions {
	opts := DefaultInputOptions(i.cfg.Format)

	// HasHeader: use config value (defaults to true via DefaultInputOptions,
	// but ImportConfig.HasHeader=false should override it)
	opts.HasHeader = i.cfg.HasHeader

	if i.cfg.Delimiter != "" {
		opts.Delimiter = rune(i.cfg.Delimiter[0])
	}

	if len(i.cfg.Columns) > 0 {
		opts.Columns = i.cfg.Columns
	}

	if len(i.cfg.NullValues) > 0 {
		opts.NullValues = i.cfg.NullValues
	}

	return opts
}

func (i *Importer) resolveColumns(reader InputReader) ([]string, error) {
	// Use explicit columns if provided
	if len(i.cfg.Columns) > 0 {
		return i.cfg.Columns, nil
	}

	// Otherwise read from header
	columns, err := reader.ReadHeader()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	return columns, nil
}

func (i *Importer) processBatches(ctx context.Context, reader InputReader, columns []string) error {
	batch := make([][]any, 0, i.cfg.BatchSize)
	skippedRows := 0
	pendingRows := int64(0) // Track rows in current batch

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check max rows limit (including pending rows in batch)
		if i.cfg.MaxRows > 0 && (i.metrics.RowsImported+pendingRows) >= int64(i.cfg.MaxRows) {
			break
		}

		// Read next row
		row, err := reader.ReadRow()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read row: %w", err)
		}

		i.metrics.RowsRead++

		// Skip rows if configured
		if skippedRows < i.cfg.SkipRows {
			skippedRows++
			i.metrics.RowsSkipped++
			continue
		}

		// Add row to batch
		batch = append(batch, row)
		pendingRows++

		// Insert batch when full
		if len(batch) >= i.cfg.BatchSize {
			if err := i.insertBatch(ctx, columns, batch); err != nil {
				return err
			}
			batch = batch[:0]
			pendingRows = 0
		}
	}

	// Insert remaining rows
	if len(batch) > 0 {
		if err := i.insertBatch(ctx, columns, batch); err != nil {
			return err
		}
	}

	return nil
}

func (i *Importer) insertBatch(ctx context.Context, columns []string, rows [][]any) error {
	if i.cfg.DryRun {
		// In dry run mode, just count the rows
		i.metrics.RowsImported += int64(len(rows))
		i.metrics.BatchCount++
		return nil
	}

	// Build the INSERT query
	query := i.driver.BuildInsertQuery(i.cfg.Table, columns, len(rows), i.cfg.OnConflict)

	// Flatten rows into parameters
	params := flattenRows(rows)

	// Execute the query
	var err error
	if i.tx != nil {
		_, err = i.tx.ExecContext(ctx, query, params...)
	} else {
		_, err = i.db.ExecContext(ctx, query, params...)
	}

	if err != nil {
		return fmt.Errorf("failed to insert batch: %w", err)
	}

	i.metrics.RowsImported += int64(len(rows))
	i.metrics.BatchCount++

	return nil
}

// flattenRows converts a 2D slice of rows into a 1D slice of parameters.
func flattenRows(rows [][]any) []any {
	if len(rows) == 0 {
		return nil
	}

	// Estimate capacity
	totalParams := 0
	for _, row := range rows {
		totalParams += len(row)
	}

	params := make([]any, 0, totalParams)
	for _, row := range rows {
		params = append(params, row...)
	}

	return params
}
