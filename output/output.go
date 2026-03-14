// Package output formats query results as JSONL, CSV, etc.
package output

import (
	"fmt"
	"io"
	"iter"
)

// QueryResult provides access to query result data for formatting.
type QueryResult interface {
	ColumnNames() []string
	Chunks() iter.Seq2[[]map[string]any, error]
}

// ResultFormatter writes query results in a specific format.
type ResultFormatter interface {
	Start(columnNames []string) error
	WriteChunk(rows []map[string]interface{}) error
	Close() error
}

// NewFormatter creates a ResultFormatter for the specified format.
func NewFormatter(w io.Writer, format string) (ResultFormatter, error) {
	switch format {
	case "json":
		return newJSONLFormatter(w), nil
	case "csv":
		return newCSVFormatter(w), nil
	default:
		return nil, fmt.Errorf("unsupported output format: %q", format)
	}
}

// WriteResult writes query results to w in the specified format.
func WriteResult(w io.Writer, result QueryResult, format string) (int, error) {
	f, err := NewFormatter(w, format)
	if err != nil {
		return 0, err
	}
	if err := f.Start(result.ColumnNames()); err != nil {
		return 0, err
	}
	rowCount := 0
	for chunk, err := range result.Chunks() {
		if err != nil {
			return 0, err
		}
		if err := f.WriteChunk(chunk); err != nil {
			return 0, err
		}
		rowCount += len(chunk)
	}
	if err := f.Close(); err != nil {
		return 0, err
	}
	return rowCount, nil
}
