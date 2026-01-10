package sql

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"
)

// JSON formatting constants
const jsonIndent = "  " // Two spaces for JSON indentation

// ResultWriter defines the interface for writing query results.
type ResultWriter interface {
	// WriteHeader writes the column headers (if applicable).
	WriteHeader(columns []string) error

	// WriteRow writes a single row of data.
	WriteRow(values []any) error

	// Flush ensures all buffered data is written.
	Flush() error

	// Close finalizes the writer.
	Close() error
}

// JSONLWriter writes results in JSON Lines format.
type JSONLWriter struct {
	w          io.Writer
	columns    []string
	nullString string
	rowCount   int
}

// NewJSONLWriter creates a new JSONL writer.
func NewJSONLWriter(w io.Writer, nullString string) *JSONLWriter {
	return &JSONLWriter{
		w:          w,
		nullString: nullString,
	}
}

// WriteHeader stores column names for row writing.
func (w *JSONLWriter) WriteHeader(columns []string) error {
	w.columns = columns
	return nil
}

// WriteRow writes a row as a JSON object.
func (w *JSONLWriter) WriteRow(values []any) error {
	row := make(map[string]any, len(values))
	for i, val := range values {
		if i < len(w.columns) {
			row[w.columns[i]] = convertValue(val, w.nullString)
		}
	}

	data, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("failed to marshal row: %w", err)
	}

	if _, err := w.w.Write(data); err != nil {
		return err
	}
	if _, err := w.w.Write([]byte("\n")); err != nil {
		return err
	}

	w.rowCount++
	return nil
}

// Flush is a no-op for JSONL writer.
func (w *JSONLWriter) Flush() error {
	return nil
}

// Close is a no-op for JSONL writer.
func (w *JSONLWriter) Close() error {
	return nil
}

// RowCount returns the number of rows written.
func (w *JSONLWriter) RowCount() int {
	return w.rowCount
}

// JSONWriter writes results as a JSON array.
type JSONWriter struct {
	w          io.Writer
	columns    []string
	nullString string
	rows       []map[string]any
}

// NewJSONWriter creates a new JSON writer.
func NewJSONWriter(w io.Writer, nullString string) *JSONWriter {
	return &JSONWriter{
		w:          w,
		nullString: nullString,
		rows:       make([]map[string]any, 0),
	}
}

// WriteHeader stores column names for row writing.
func (w *JSONWriter) WriteHeader(columns []string) error {
	w.columns = columns
	return nil
}

// WriteRow accumulates a row for later writing.
func (w *JSONWriter) WriteRow(values []any) error {
	row := make(map[string]any, len(values))
	for i, val := range values {
		if i < len(w.columns) {
			row[w.columns[i]] = convertValue(val, w.nullString)
		}
	}
	w.rows = append(w.rows, row)
	return nil
}

// Flush is a no-op for JSON writer.
func (w *JSONWriter) Flush() error {
	return nil
}

// Close writes the accumulated JSON array.
func (w *JSONWriter) Close() error {
	data, err := json.MarshalIndent(w.rows, "", jsonIndent)
	if err != nil {
		return fmt.Errorf("failed to marshal rows: %w", err)
	}

	if _, err := w.w.Write(data); err != nil {
		return err
	}
	if _, err := w.w.Write([]byte("\n")); err != nil {
		return err
	}

	return nil
}

// RowCount returns the number of rows accumulated.
func (w *JSONWriter) RowCount() int {
	return len(w.rows)
}

// CSVWriter writes results in CSV format.
type CSVWriter struct {
	w           *csv.Writer
	columns     []string
	nullString  string
	writeHeader bool
	rowCount    int
}

// NewCSVWriter creates a new CSV writer.
func NewCSVWriter(w io.Writer, nullString string, writeHeader bool) *CSVWriter {
	return &CSVWriter{
		w:           csv.NewWriter(w),
		nullString:  nullString,
		writeHeader: writeHeader,
	}
}

// WriteHeader writes the CSV header row if enabled.
func (w *CSVWriter) WriteHeader(columns []string) error {
	w.columns = columns
	if w.writeHeader {
		return w.w.Write(columns)
	}
	return nil
}

// WriteRow writes a row of values as CSV.
func (w *CSVWriter) WriteRow(values []any) error {
	record := make([]string, len(values))
	for i, val := range values {
		record[i] = valueToString(val, w.nullString)
	}
	w.rowCount++
	return w.w.Write(record)
}

// Flush writes any buffered data.
func (w *CSVWriter) Flush() error {
	w.w.Flush()
	return w.w.Error()
}

// Close flushes the writer.
func (w *CSVWriter) Close() error {
	return w.Flush()
}

// RowCount returns the number of rows written.
func (w *CSVWriter) RowCount() int {
	return w.rowCount
}

// NewResultWriter creates a ResultWriter based on the output format.
func NewResultWriter(w io.Writer, format, nullString string, headers bool) ResultWriter {
	switch format {
	case "json":
		return NewJSONWriter(w, nullString)
	case "csv":
		return NewCSVWriter(w, nullString, headers)
	default:
		return NewJSONLWriter(w, nullString)
	}
}

// convertValue converts a SQL value to a JSON-compatible type.
// For JSON/JSONL output, NULL values are always converted to nil (JSON null).
// The nullString parameter is kept for compatibility but ignored for JSON output.
func convertValue(val any, _ string) any {
	if val == nil {
		return nil // Always return nil for JSON null
	}

	switch v := val.(type) {
	case []byte:
		return string(v)
	case time.Time:
		return v.Format(time.RFC3339)
	case sql.NullString:
		if v.Valid {
			return v.String
		}
		return nil
	case sql.NullInt64:
		if v.Valid {
			return v.Int64
		}
		return nil
	case sql.NullFloat64:
		if v.Valid {
			return v.Float64
		}
		return nil
	case sql.NullBool:
		if v.Valid {
			return v.Bool
		}
		return nil
	case sql.NullTime:
		if v.Valid {
			return v.Time.Format(time.RFC3339)
		}
		return nil
	default:
		return v
	}
}

// valueToString converts a value to its string representation for CSV.
func valueToString(val any, nullString string) string {
	if val == nil {
		return nullString
	}

	switch v := val.(type) {
	case []byte:
		return string(v)
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case time.Time:
		return v.Format(time.RFC3339)
	case sql.NullString:
		if v.Valid {
			return v.String
		}
		return nullString
	case sql.NullInt64:
		if v.Valid {
			return strconv.FormatInt(v.Int64, 10)
		}
		return nullString
	case sql.NullFloat64:
		if v.Valid {
			return strconv.FormatFloat(v.Float64, 'f', -1, 64)
		}
		return nullString
	case sql.NullBool:
		if v.Valid {
			return strconv.FormatBool(v.Bool)
		}
		return nullString
	case sql.NullTime:
		if v.Valid {
			return v.Time.Format(time.RFC3339)
		}
		return nullString
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ScanRow scans a row into a slice of interface values.
func ScanRow(rows *sql.Rows, columns []string) ([]any, error) {
	if rows == nil {
		return nil, fmt.Errorf("rows is nil")
	}

	values := make([]any, len(columns))
	valuePtrs := make([]any, len(columns))

	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	return values, nil
}
