package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlattenRows(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		rows [][]any
		want []any
	}{
		{
			name: "single row",
			rows: [][]any{
				{"Alice", 30, "NYC"},
			},
			want: []any{"Alice", 30, "NYC"},
		},
		{
			name: "multiple rows",
			rows: [][]any{
				{"Alice", 30},
				{"Bob", 25},
				{"Charlie", 35},
			},
			want: []any{"Alice", 30, "Bob", 25, "Charlie", 35},
		},
		{
			name: "empty rows",
			rows: [][]any{},
			want: nil,
		},
		{
			name: "rows with nil values",
			rows: [][]any{
				{"Alice", nil, "NYC"},
				{nil, 25, nil},
			},
			want: []any{"Alice", nil, "NYC", nil, 25, nil},
		},
		{
			name: "single column rows",
			rows: [][]any{
				{1},
				{2},
				{3},
			},
			want: []any{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := flattenRows(tt.rows)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildInputOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		cfg       *ImportConfig
		wantDelim rune
	}{
		{
			name: "csv with default delimiter",
			cfg: &ImportConfig{
				Format:    "csv",
				HasHeader: new(true),
			},
			wantDelim: ',',
		},
		{
			name: "tsv with default delimiter",
			cfg: &ImportConfig{
				Format:    "tsv",
				HasHeader: new(true),
			},
			wantDelim: '\t',
		},
		{
			name: "custom delimiter",
			cfg: &ImportConfig{
				Format:    "csv",
				Delimiter: ";",
			},
			wantDelim: ';',
		},
		{
			name: "csv with custom columns",
			cfg: &ImportConfig{
				Format:  "csv",
				Columns: []string{"a", "b", "c"},
			},
			wantDelim: ',',
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			importer := &Importer{cfg: tt.cfg}
			opts := importer.buildInputOptions()
			assert.Equal(t, tt.wantDelim, opts.Delimiter)
		})
	}
}

func TestImportMetrics(t *testing.T) {
	t.Parallel()
	metrics := &ImportMetrics{
		RowsRead:     100,
		RowsImported: 95,
		RowsSkipped:  5,
		BatchCount:   10,
		Status:       "completed",
	}

	assert.Equal(t, int64(100), metrics.RowsRead)
	assert.Equal(t, int64(95), metrics.RowsImported)
	assert.Equal(t, int64(5), metrics.RowsSkipped)
	assert.Equal(t, 10, metrics.BatchCount)
	assert.Equal(t, "completed", metrics.Status)
}
