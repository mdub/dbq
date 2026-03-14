package output

import (
	"fmt"
	"io"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/ipc"
)

// arrowStreamFormatter writes record batches in Arrow IPC streaming format.
type arrowStreamFormatter struct {
	w  io.Writer
	iw *ipc.Writer
}

func newArrowStreamFormatter(w io.Writer) *arrowStreamFormatter {
	return &arrowStreamFormatter{w: w}
}

func (f *arrowStreamFormatter) Header(_ []string) error { return nil }

func (f *arrowStreamFormatter) Rows(batch arrow.RecordBatch) error {
	if f.iw == nil {
		f.iw = ipc.NewWriter(f.w, ipc.WithSchema(batch.Schema()))
	}
	return f.iw.Write(batch)
}

func (f *arrowStreamFormatter) Footer() error {
	if f.iw != nil {
		return f.iw.Close()
	}
	return nil
}

// arrowFileFormatter writes record batches in Arrow IPC file format.
type arrowFileFormatter struct {
	w  io.Writer
	fw *ipc.FileWriter
}

func newArrowFileFormatter(w io.Writer) *arrowFileFormatter {
	return &arrowFileFormatter{w: w}
}

func (f *arrowFileFormatter) Header(_ []string) error { return nil }

func (f *arrowFileFormatter) Rows(batch arrow.RecordBatch) error {
	if f.fw == nil {
		fw, err := ipc.NewFileWriter(f.w, ipc.WithSchema(batch.Schema()))
		if err != nil {
			return fmt.Errorf("creating Arrow IPC file writer: %w", err)
		}
		f.fw = fw
	}
	return f.fw.Write(batch)
}

func (f *arrowFileFormatter) Footer() error {
	if f.fw != nil {
		return f.fw.Close()
	}
	return nil
}
