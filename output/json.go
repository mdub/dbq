package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/apache/arrow-go/v18/arrow"
)

// jsonFormatter writes rows as a JSON array.
type jsonFormatter struct {
	w    io.Writer
	enc  *json.Encoder
	rows []map[string]interface{}
}

func newJSONFormatter(w io.Writer) *jsonFormatter {
	return &jsonFormatter{
		w:    w,
		enc:  json.NewEncoder(w),
		rows: make([]map[string]interface{}, 0),
	}
}

func (f *jsonFormatter) Columns(_ []string) error { return nil }

func (f *jsonFormatter) Rows(batch arrow.RecordBatch) error {
	f.rows = append(f.rows, ArrowToGo(batch)...)
	return nil
}

func (f *jsonFormatter) Close() error {
	if err := f.enc.Encode(f.rows); err != nil {
		return fmt.Errorf("JSON write error: %w", err)
	}
	return nil
}
