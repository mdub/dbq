package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// jsonlFormatter writes rows as newline-delimited JSON.
type jsonlFormatter struct {
	enc *json.Encoder
}

func newJSONLFormatter(w io.Writer) *jsonlFormatter {
	return &jsonlFormatter{enc: json.NewEncoder(w)}
}

func (f *jsonlFormatter) Start(_ []string) error { return nil }

func (f *jsonlFormatter) WriteChunk(rows []map[string]interface{}) error {
	for _, row := range rows {
		if err := f.enc.Encode(row); err != nil {
			return fmt.Errorf("JSONL write error: %w", err)
		}
	}
	return nil
}

func (f *jsonlFormatter) Close() error { return nil }
