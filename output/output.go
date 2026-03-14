// Package output formats query results as JSONL, CSV, etc.
package output

import (
	"fmt"
	"io"

	"github.com/mdub/dbq/result"
)

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
func WriteResult(w io.Writer, qr result.QueryResult, format string) (int, error) {
	f, err := NewFormatter(w, format)
	if err != nil {
		return 0, err
	}
	started := false
	rowCount := 0
	for rec, err := range qr.Chunks() {
		if err != nil {
			return 0, err
		}
		if !started {
			columnNames := make([]string, rec.Schema().NumFields())
			for i, field := range rec.Schema().Fields() {
				columnNames[i] = field.Name
			}
			if err := f.Start(columnNames); err != nil {
				return 0, err
			}
			started = true
		}
		rows := ArrowToGo(rec)
		if err := f.WriteChunk(rows); err != nil {
			return 0, err
		}
		rowCount += int(rec.NumRows())
	}
	if !started {
		// No chunks at all; start with empty column list
		if err := f.Start(nil); err != nil {
			return 0, err
		}
	}
	if err := f.Close(); err != nil {
		return 0, err
	}
	return rowCount, nil
}
