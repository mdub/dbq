package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
)

// ResultFormatter writes query results in a specific format.
type ResultFormatter interface {
	Start(columnNames []string) error
	WriteChunk(rows []map[string]interface{}) error
	Close() error
}

// newResultFormatter creates a ResultFormatter for the specified format.
func newResultFormatter(w io.Writer, format string) (ResultFormatter, error) {
	switch format {
	case "json":
		return &jsonlFormatter{enc: json.NewEncoder(w)}, nil
	case "csv":
		return &csvFormatter{w: w}, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %q", format)
	}
}

// writeResult writes query results to w in the specified format.
func writeResult(w io.Writer, result QueryResult, format string) (int, error) {
	f, err := newResultFormatter(w, format)
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

// jsonlFormatter writes rows as newline-delimited JSON.
type jsonlFormatter struct {
	enc *json.Encoder
}

func (f *jsonlFormatter) Start(columnNames []string) error { return nil }

func (f *jsonlFormatter) WriteChunk(rows []map[string]interface{}) error {
	for _, row := range rows {
		if err := f.enc.Encode(row); err != nil {
			return fmt.Errorf("JSONL write error: %w", err)
		}
	}
	return nil
}

func (f *jsonlFormatter) Close() error { return nil }

// csvFormatter writes rows as CSV with a header row.
type csvFormatter struct {
	w       io.Writer
	cw      *csv.Writer
	columns []string
}

func (f *csvFormatter) Start(columnNames []string) error {
	f.columns = columnNames
	f.cw = csv.NewWriter(f.w)
	if len(columnNames) > 0 {
		f.cw.Write(columnNames)
	}
	return nil
}

func (f *csvFormatter) WriteChunk(rows []map[string]interface{}) error {
	for _, row := range rows {
		record := make([]string, len(f.columns))
		for i, name := range f.columns {
			record[i] = formatCellValue(row[name])
		}
		f.cw.Write(record)
	}
	f.cw.Flush()
	if err := f.cw.Error(); err != nil {
		return fmt.Errorf("CSV write error: %w", err)
	}
	return nil
}

// formatCellValue converts a value to a string for CSV output.
// Structured values (maps, slices) are JSON-encoded.
func formatCellValue(v interface{}) string {
	switch v.(type) {
	case map[string]interface{}, []interface{}:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (f *csvFormatter) Close() error {
	f.cw.Flush()
	return f.cw.Error()
}
