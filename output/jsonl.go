package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/apache/arrow-go/v18/arrow"
)

// jsonlFormatter writes rows as newline-delimited JSON.
type jsonlFormatter struct {
	enc *json.Encoder
}

func newJSONLFormatter(w io.Writer) *jsonlFormatter {
	return &jsonlFormatter{enc: json.NewEncoder(w)}
}

func (f *jsonlFormatter) WriteRecordBatch(batch arrow.RecordBatch) error {
	for _, row := range ArrowToGo(batch) {
		if err := f.enc.Encode(row); err != nil {
			return fmt.Errorf("JSONL write error: %w", err)
		}
	}
	return nil
}

func (f *jsonlFormatter) Close() error { return nil }
