package output

import (
	"fmt"
	"io"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

// parquetFormatter writes record batches as Parquet.
type parquetFormatter struct {
	w  io.Writer
	pw *pqarrow.FileWriter
}

func newParquetFormatter(w io.Writer) *parquetFormatter {
	return &parquetFormatter{w: w}
}

func (f *parquetFormatter) WriteRecordBatch(batch arrow.RecordBatch) error {
	if f.pw == nil {
		pw, err := pqarrow.NewFileWriter(batch.Schema(), f.w, nil, pqarrow.DefaultWriterProps())
		if err != nil {
			return fmt.Errorf("creating Parquet writer: %w", err)
		}
		f.pw = pw
	}
	return f.pw.WriteBuffered(batch)
}

func (f *parquetFormatter) Close() error {
	if f.pw != nil {
		return f.pw.Close()
	}
	return nil
}
