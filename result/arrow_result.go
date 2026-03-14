package result

import (
	"fmt"
	"io"
	"iter"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/ipc"
)

// readArrowStream reads an Arrow IPC stream and yields record batches.
func readArrowStream(r io.Reader) iter.Seq2[arrow.RecordBatch, error] {
	return func(yield func(arrow.RecordBatch, error) bool) {
		reader, err := ipc.NewReader(r)
		if err != nil {
			yield(nil, fmt.Errorf("reading Arrow stream: %w", err))
			return
		}
		defer reader.Release()

		for reader.Next() {
			if !yield(reader.RecordBatch(), nil) {
				return
			}
		}
		if err := reader.Err(); err != nil {
			yield(nil, err)
		}
	}
}
