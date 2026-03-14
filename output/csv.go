package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"github.com/apache/arrow-go/v18/arrow"
)

// csvFormatter writes rows as CSV with a header row.
type csvFormatter struct {
	w       io.Writer
	cw      *csv.Writer
	columns []string
}

func newCSVFormatter(w io.Writer) *csvFormatter {
	return &csvFormatter{w: w, cw: csv.NewWriter(w)}
}

func (f *csvFormatter) Header(columnNames []string) error {
	f.columns = columnNames
	if len(columnNames) > 0 {
		_ = f.cw.Write(columnNames)
	}
	return nil
}

func (f *csvFormatter) Rows(batch arrow.RecordBatch) error {
	for _, row := range ArrowToGo(batch) {
		record := make([]string, len(f.columns))
		for i, name := range f.columns {
			record[i] = formatCellValue(row[name])
		}
		_ = f.cw.Write(record)
	}
	f.cw.Flush()
	if err := f.cw.Error(); err != nil {
		return fmt.Errorf("CSV write error: %w", err)
	}
	return nil
}

func (f *csvFormatter) Footer() error {
	f.cw.Flush()
	return f.cw.Error()
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
