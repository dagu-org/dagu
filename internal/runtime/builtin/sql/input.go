package sql

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

// Scanner buffer sizes for JSONL reading
const (
	defaultScannerBufSize = 64 * 1024   // 64KB initial buffer
	maxScannerBufSize     = 1024 * 1024 // 1MB max buffer for large JSON lines
)

// InputReader defines the interface for reading input data for import.
// This is the symmetric inverse of ResultWriter - it reads rows for input
// rather than writing rows for output.
type InputReader interface {
	// ReadHeader reads the column headers from the input.
	// Returns nil for formats without headers or when hasHeader is false.
	ReadHeader() ([]string, error)

	// ReadRow reads the next row of data.
	// Returns io.EOF when no more rows are available.
	ReadRow() ([]any, error)

	// Close releases any resources held by the reader.
	Close() error
}

// InputOptions configures input reader behavior.
type InputOptions struct {
	HasHeader  bool     // Whether first row is header (CSV/TSV)
	Delimiter  rune     // Field delimiter (default: ',' for CSV, '\t' for TSV)
	NullValues []string // Values to treat as NULL
	Columns    []string // Expected column names (for JSONL)
}

// DefaultInputOptions returns default options for the given format.
func DefaultInputOptions(format string) InputOptions {
	opts := InputOptions{
		HasHeader:  true,
		NullValues: []string{"", "NULL", "null", "\\N"},
	}
	switch strings.ToLower(format) {
	case "tsv":
		opts.Delimiter = '\t'
	default:
		opts.Delimiter = ','
	}
	return opts
}

// NewInputReader creates an InputReader based on the input format.
func NewInputReader(r io.Reader, format string, opts InputOptions) (InputReader, error) {
	switch strings.ToLower(format) {
	case "csv", "tsv":
		return NewCSVReader(r, opts), nil
	case "jsonl":
		return NewJSONLReader(r, opts), nil
	default:
		return nil, fmt.Errorf("unsupported input format: %s", format)
	}
}

// DetectFormat attempts to detect the format from a file path.
func DetectFormat(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".csv":
		return "csv"
	case ".tsv":
		return "tsv"
	case ".jsonl", ".ndjson":
		return "jsonl"
	default:
		return "csv" // default to CSV
	}
}

// CSVReader implements InputReader for CSV/TSV formatted input.
type CSVReader struct {
	r          *csv.Reader
	columns    []string
	hasHeader  bool
	nullValues map[string]bool
	headerRead bool
}

// NewCSVReader creates a new CSV/TSV reader.
func NewCSVReader(r io.Reader, opts InputOptions) *CSVReader {
	csvReader := csv.NewReader(r)
	csvReader.Comma = opts.Delimiter
	csvReader.LazyQuotes = true
	csvReader.TrimLeadingSpace = true

	nullValues := make(map[string]bool)
	for _, v := range opts.NullValues {
		nullValues[v] = true
	}

	return &CSVReader{
		r:          csvReader,
		hasHeader:  opts.HasHeader,
		nullValues: nullValues,
		columns:    opts.Columns,
	}
}

// ReadHeader reads the header row from CSV.
func (r *CSVReader) ReadHeader() ([]string, error) {
	if r.headerRead {
		return r.columns, nil
	}
	r.headerRead = true

	if !r.hasHeader {
		return r.columns, nil
	}

	record, err := r.r.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	r.columns = record
	return r.columns, nil
}

// ReadRow reads the next data row from CSV.
func (r *CSVReader) ReadRow() ([]any, error) {
	// Ensure header is read first if present
	if !r.headerRead && r.hasHeader {
		if _, err := r.ReadHeader(); err != nil {
			return nil, err
		}
	}

	record, err := r.r.Read()
	if err != nil {
		return nil, err // includes io.EOF
	}

	// Convert string slice to any slice, handling NULL values
	row := make([]any, len(record))
	for i, val := range record {
		if r.nullValues[val] {
			row[i] = nil
		} else {
			row[i] = val
		}
	}

	return row, nil
}

// Close is a no-op for CSVReader (underlying reader should be closed by caller).
func (r *CSVReader) Close() error {
	return nil
}

// JSONLReader implements InputReader for JSON Lines formatted input.
type JSONLReader struct {
	scanner      *bufio.Scanner
	columns      []string
	nullValues   map[string]bool
	headerRead   bool
	firstRow     []any // Preserved first row when columns are auto-detected
	firstRowRead bool  // Whether the preserved first row has been returned
}

// NewJSONLReader creates a new JSON Lines reader.
func NewJSONLReader(r io.Reader, opts InputOptions) *JSONLReader {
	nullValues := make(map[string]bool)
	for _, v := range opts.NullValues {
		nullValues[v] = true
	}

	scanner := bufio.NewScanner(r)
	// Increase buffer size for large JSON lines
	scanner.Buffer(make([]byte, defaultScannerBufSize), maxScannerBufSize)

	return &JSONLReader{
		scanner:    scanner,
		columns:    opts.Columns,
		nullValues: nullValues,
	}
}

// ReadHeader returns the expected columns for JSONL.
// If columns were not specified, reads the first line to determine keys.
func (r *JSONLReader) ReadHeader() ([]string, error) {
	if r.headerRead {
		return r.columns, nil
	}
	r.headerRead = true

	// If columns were explicitly provided, use them
	if len(r.columns) > 0 {
		return r.columns, nil
	}

	// Read first line to determine column order
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read JSONL: %w", err)
		}
		return nil, io.EOF
	}

	line := r.scanner.Text()
	if line == "" {
		return nil, fmt.Errorf("empty first line in JSONL")
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return nil, fmt.Errorf("failed to parse JSONL header line: %w", err)
	}

	// Extract keys and sort for deterministic ordering
	r.columns = extractSortedKeys(obj)

	// Preserve the first row data so it can be returned by ReadRow
	r.firstRow = make([]any, len(r.columns))
	for i, col := range r.columns {
		val, exists := obj[col]
		if !exists {
			r.firstRow[i] = nil
			continue
		}
		if strVal, ok := val.(string); ok && r.nullValues[strVal] {
			r.firstRow[i] = nil
		} else {
			r.firstRow[i] = val
		}
	}

	return r.columns, nil
}

// ReadRow reads the next JSON object from the input.
func (r *JSONLReader) ReadRow() ([]any, error) {
	// Return preserved first row if it exists and hasn't been returned yet
	if r.firstRow != nil && !r.firstRowRead {
		r.firstRowRead = true
		return r.firstRow, nil
	}

	// Read lines until we get a non-empty one (iterative, not recursive)
	var line string
	for {
		if !r.scanner.Scan() {
			if err := r.scanner.Err(); err != nil {
				return nil, fmt.Errorf("failed to read JSONL: %w", err)
			}
			return nil, io.EOF
		}
		line = r.scanner.Text()
		if line != "" {
			break
		}
		// Skip empty lines and continue loop
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return nil, fmt.Errorf("failed to parse JSONL row: %w", err)
	}

	// Build row in column order
	row := make([]any, len(r.columns))
	for i, col := range r.columns {
		val, exists := obj[col]
		if !exists {
			row[i] = nil
			continue
		}

		// Handle null values represented as strings
		if strVal, ok := val.(string); ok && r.nullValues[strVal] {
			row[i] = nil
		} else {
			row[i] = val
		}
	}

	return row, nil
}

// Close is a no-op for JSONLReader.
func (r *JSONLReader) Close() error {
	return nil
}

// extractSortedKeys extracts keys from a map and returns them in sorted order.
func extractSortedKeys(obj map[string]any) []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
