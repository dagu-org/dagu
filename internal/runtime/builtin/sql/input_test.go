package sql

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/file.csv", "csv"},
		{"/path/to/file.CSV", "csv"},
		{"/path/to/file.tsv", "tsv"},
		{"/path/to/file.TSV", "tsv"},
		{"/path/to/file.jsonl", "jsonl"},
		{"/path/to/file.JSONL", "jsonl"},
		{"/path/to/file.ndjson", "jsonl"},
		{"/path/to/file.txt", "csv"}, // default to csv
		{"/path/to/file", "csv"},     // no extension defaults to csv
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := DetectFormat(tt.path)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestDefaultInputOptions(t *testing.T) {
	tests := []struct {
		format    string
		delimiter rune
	}{
		{"csv", ','},
		{"CSV", ','},
		{"tsv", '\t'},
		{"TSV", '\t'},
		{"jsonl", ','}, // delimiter not used but defaults to comma
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			opts := DefaultInputOptions(tt.format)
			assert.Equal(t, tt.delimiter, opts.Delimiter)
			assert.True(t, opts.HasHeader)
			assert.Contains(t, opts.NullValues, "NULL")
			assert.Contains(t, opts.NullValues, "null")
		})
	}
}

func TestNewInputReader(t *testing.T) {
	tests := []struct {
		format  string
		wantErr bool
	}{
		{"csv", false},
		{"CSV", false},
		{"tsv", false},
		{"jsonl", false},
		{"xml", true}, // unsupported
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			// Create a fresh reader for each subtest to avoid exhaustion
			r := strings.NewReader("a,b\n1,2\n")
			opts := DefaultInputOptions("csv")

			reader, err := NewInputReader(r, tt.format, opts)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, reader)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, reader)
			}
		})
	}
}

func TestCSVReader_BasicRead(t *testing.T) {
	input := "name,age,city\nAlice,30,NYC\nBob,25,LA\n"
	r := strings.NewReader(input)
	opts := InputOptions{
		HasHeader: true,
		Delimiter: ',',
	}

	reader := NewCSVReader(r, opts)

	// Read header
	header, err := reader.ReadHeader()
	require.NoError(t, err)
	assert.Equal(t, []string{"name", "age", "city"}, header)

	// Read first row
	row1, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Alice", "30", "NYC"}, row1)

	// Read second row
	row2, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Bob", "25", "LA"}, row2)

	// EOF
	_, err = reader.ReadRow()
	assert.Equal(t, io.EOF, err)

	// Close
	assert.NoError(t, reader.Close())
}

func TestCSVReader_NoHeader(t *testing.T) {
	input := "Alice,30,NYC\nBob,25,LA\n"
	r := strings.NewReader(input)
	opts := InputOptions{
		HasHeader: false,
		Delimiter: ',',
		Columns:   []string{"name", "age", "city"},
	}

	reader := NewCSVReader(r, opts)

	// ReadHeader should return provided columns without consuming a row
	header, err := reader.ReadHeader()
	require.NoError(t, err)
	assert.Equal(t, []string{"name", "age", "city"}, header)

	// First row should be data, not header
	row1, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Alice", "30", "NYC"}, row1)

	// Second row
	row2, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Bob", "25", "LA"}, row2)
}

func TestCSVReader_NullValues(t *testing.T) {
	input := "name,value\nAlice,\nBob,NULL\nCharlie,null\nDave,\\N\nEve,actual\n"
	r := strings.NewReader(input)
	opts := InputOptions{
		HasHeader:  true,
		Delimiter:  ',',
		NullValues: []string{"", "NULL", "null", "\\N"},
	}

	reader := NewCSVReader(r, opts)

	_, err := reader.ReadHeader()
	require.NoError(t, err)

	// Empty string -> nil
	row1, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Alice", nil}, row1)

	// NULL -> nil
	row2, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Bob", nil}, row2)

	// null -> nil
	row3, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Charlie", nil}, row3)

	// \N -> nil
	row4, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Dave", nil}, row4)

	// actual value
	row5, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Eve", "actual"}, row5)
}

func TestCSVReader_TSV(t *testing.T) {
	input := "name\tage\tcity\nAlice\t30\tNYC\n"
	r := strings.NewReader(input)
	opts := InputOptions{
		HasHeader: true,
		Delimiter: '\t',
	}

	reader := NewCSVReader(r, opts)

	header, err := reader.ReadHeader()
	require.NoError(t, err)
	assert.Equal(t, []string{"name", "age", "city"}, header)

	row, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Alice", "30", "NYC"}, row)
}

func TestCSVReader_QuotedFields(t *testing.T) {
	input := `name,description
"Alice","Hello, World"
"Bob","Line1
Line2"
`
	r := strings.NewReader(input)
	opts := InputOptions{
		HasHeader: true,
		Delimiter: ',',
	}

	reader := NewCSVReader(r, opts)

	_, err := reader.ReadHeader()
	require.NoError(t, err)

	// Comma in quoted field
	row1, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Alice", "Hello, World"}, row1)

	// Newline in quoted field
	row2, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Bob", "Line1\nLine2"}, row2)
}

func TestCSVReader_ReadRowAutoHeader(t *testing.T) {
	// Test that ReadRow automatically reads header if not done
	input := "name,age\nAlice,30\n"
	r := strings.NewReader(input)
	opts := InputOptions{
		HasHeader: true,
		Delimiter: ',',
	}

	reader := NewCSVReader(r, opts)

	// Don't call ReadHeader, go straight to ReadRow
	row, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Alice", "30"}, row)
}

func TestCSVReader_EmptyInput(t *testing.T) {
	r := strings.NewReader("")
	opts := InputOptions{
		HasHeader: true,
		Delimiter: ',',
	}

	reader := NewCSVReader(r, opts)

	_, err := reader.ReadHeader()
	assert.Error(t, err) // EOF or parse error
}

func TestJSONLReader_BasicRead(t *testing.T) {
	input := `{"name":"Alice","age":30,"city":"NYC"}
{"name":"Bob","age":25,"city":"LA"}
`
	r := strings.NewReader(input)
	opts := InputOptions{
		Columns: []string{"name", "age", "city"},
	}

	reader := NewJSONLReader(r, opts)

	// Read header (columns were provided)
	header, err := reader.ReadHeader()
	require.NoError(t, err)
	assert.Equal(t, []string{"name", "age", "city"}, header)

	// Read first row
	row1, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, "Alice", row1[0])
	assert.Equal(t, float64(30), row1[1]) // JSON numbers are float64
	assert.Equal(t, "NYC", row1[2])

	// Read second row
	row2, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, "Bob", row2[0])
	assert.Equal(t, float64(25), row2[1])
	assert.Equal(t, "LA", row2[2])

	// EOF
	_, err = reader.ReadRow()
	assert.Equal(t, io.EOF, err)
}

func TestJSONLReader_NullValues(t *testing.T) {
	input := `{"name":"Alice","value":null}
{"name":"Bob","value":"NULL"}
`
	r := strings.NewReader(input)
	opts := InputOptions{
		Columns:    []string{"name", "value"},
		NullValues: []string{"NULL"},
	}

	reader := NewJSONLReader(r, opts)
	reader.ReadHeader()

	// JSON null -> nil
	row1, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, "Alice", row1[0])
	assert.Nil(t, row1[1])

	// String "NULL" -> nil (via NullValues)
	row2, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, "Bob", row2[0])
	assert.Nil(t, row2[1])
}

func TestJSONLReader_MissingFields(t *testing.T) {
	input := `{"name":"Alice"}
`
	r := strings.NewReader(input)
	opts := InputOptions{
		Columns: []string{"name", "age"},
	}

	reader := NewJSONLReader(r, opts)
	reader.ReadHeader()

	row, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, "Alice", row[0])
	assert.Nil(t, row[1]) // Missing field is nil
}

func TestJSONLReader_SkipEmptyLines(t *testing.T) {
	input := `{"name":"Alice"}

{"name":"Bob"}
`
	r := strings.NewReader(input)
	opts := InputOptions{
		Columns: []string{"name"},
	}

	reader := NewJSONLReader(r, opts)
	reader.ReadHeader()

	row1, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, "Alice", row1[0])

	// Empty line should be skipped
	row2, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, "Bob", row2[0])
}

func TestJSONLReader_InvalidJSON(t *testing.T) {
	input := `{"name":"Alice"}
not valid json
`
	r := strings.NewReader(input)
	opts := InputOptions{
		Columns: []string{"name"},
	}

	reader := NewJSONLReader(r, opts)
	reader.ReadHeader()

	_, err := reader.ReadRow()
	require.NoError(t, err)

	// Invalid JSON should return error
	_, err = reader.ReadRow()
	assert.Error(t, err)
}

func TestJSONLReader_AutoDetectColumns(t *testing.T) {
	input := `{"name":"Alice","age":30}
{"name":"Bob","age":25}
`
	r := strings.NewReader(input)
	opts := InputOptions{}

	reader := NewJSONLReader(r, opts)

	// Read header - this auto-detects columns and preserves first row
	columns, err := reader.ReadHeader()
	require.NoError(t, err)
	assert.Len(t, columns, 2)

	// First ReadRow should return the first row (not skip it)
	row1, err := reader.ReadRow()
	require.NoError(t, err)
	// Find name and age positions
	var nameVal, ageVal any
	for i, col := range columns {
		if col == "name" {
			nameVal = row1[i]
		} else if col == "age" {
			ageVal = row1[i]
		}
	}
	assert.Equal(t, "Alice", nameVal)
	assert.Equal(t, float64(30), ageVal)

	// Second row
	row2, err := reader.ReadRow()
	require.NoError(t, err)
	for i, col := range columns {
		if col == "name" {
			nameVal = row2[i]
		} else if col == "age" {
			ageVal = row2[i]
		}
	}
	assert.Equal(t, "Bob", nameVal)
	assert.Equal(t, float64(25), ageVal)

	// EOF
	_, err = reader.ReadRow()
	assert.Equal(t, io.EOF, err)
}

func TestJSONLReader_EmptyInput(t *testing.T) {
	r := strings.NewReader("")
	opts := InputOptions{}

	reader := NewJSONLReader(r, opts)
	_, err := reader.ReadHeader()
	assert.Error(t, err) // Should fail on empty input
}

func TestJSONLReader_InvalidFirstLine(t *testing.T) {
	r := strings.NewReader("not json\n")
	opts := InputOptions{}

	reader := NewJSONLReader(r, opts)
	_, err := reader.ReadHeader()
	assert.Error(t, err)
}

func TestCSVReader_HeaderReadOnce(t *testing.T) {
	input := "name,age\nAlice,30\nBob,25\n"
	r := strings.NewReader(input)
	opts := InputOptions{
		HasHeader: true,
		Delimiter: ',',
	}

	reader := NewCSVReader(r, opts)

	// Read header multiple times
	header1, err := reader.ReadHeader()
	require.NoError(t, err)
	assert.Equal(t, []string{"name", "age"}, header1)

	header2, err := reader.ReadHeader()
	require.NoError(t, err)
	assert.Equal(t, []string{"name", "age"}, header2)

	// First row should still be Alice (header was only consumed once)
	row, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Alice", "30"}, row)
}

func TestJSONLReader_HeaderReadOnce(t *testing.T) {
	input := `{"name":"Alice"}
{"name":"Bob"}
`
	r := strings.NewReader(input)
	opts := InputOptions{
		Columns: []string{"name"},
	}

	reader := NewJSONLReader(r, opts)

	// Read header multiple times
	header1, err := reader.ReadHeader()
	require.NoError(t, err)
	assert.Equal(t, []string{"name"}, header1)

	header2, err := reader.ReadHeader()
	require.NoError(t, err)
	assert.Equal(t, []string{"name"}, header2)
}

func TestCSVReader_LeadingWhitespace(t *testing.T) {
	input := "name, age\nAlice, 30\n"
	r := strings.NewReader(input)
	opts := InputOptions{
		HasHeader: true,
		Delimiter: ',',
	}

	reader := NewCSVReader(r, opts)

	header, err := reader.ReadHeader()
	require.NoError(t, err)
	// TrimLeadingSpace is enabled, so spaces should be trimmed
	assert.Equal(t, []string{"name", "age"}, header)

	row, err := reader.ReadRow()
	require.NoError(t, err)
	assert.Equal(t, []any{"Alice", "30"}, row)
}
