// Package output formats query results as JSONL, CSV, etc.
package output

import (
	"fmt"
	"io"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/mdub/dbq/result"
)

// ResultFormatter writes query results in a specific format.
type ResultFormatter interface {
	Header(names []string) error
	Rows(batch arrow.RecordBatch) error
	Footer() error
}

// NewFormatter creates a ResultFormatter for the specified format.
func NewFormatter(w io.Writer, format string) (ResultFormatter, error) {
	switch format {
	case "arrow":
		return newArrowFileFormatter(w), nil
	case "arrows":
		return newArrowStreamFormatter(w), nil
	case "csv":
		return newCSVFormatter(w), nil
	case "json":
		return newJSONFormatter(w), nil
	case "jsonl":
		return newJSONLFormatter(w), nil
	case "parquet":
		return newParquetFormatter(w), nil
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
	if err := f.Header(qr.ColumnNames()); err != nil {
		return 0, err
	}
	rowCount := 0
	for batch, err := range qr.Chunks() {
		if err != nil {
			return 0, err
		}
		if err := f.Rows(batch); err != nil {
			return 0, err
		}
		rowCount += int(batch.NumRows())
	}
	if err := f.Footer(); err != nil {
		return 0, err
	}
	return rowCount, nil
}
