package output

import (
	"bytes"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

func TestArrowStreamFormatter(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "x", Type: arrow.PrimitiveTypes.Int64},
	}, nil)
	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.Int64Builder).Append(42)
		b.Field(0).(*array.Int64Builder).Append(99)
	})
	defer rec.Release()

	var buf bytes.Buffer
	f := newArrowStreamFormatter(&buf)
	if err := f.Header(nil); err != nil {
		t.Fatal(err)
	}
	if err := f.Rows(rec); err != nil {
		t.Fatal(err)
	}
	if err := f.Footer(); err != nil {
		t.Fatal(err)
	}

	// Verify by reading back
	reader, err := ipc.NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("invalid Arrow stream: %v", err)
	}
	defer reader.Release()

	if !reader.Next() {
		t.Fatal("expected at least one record batch")
	}
	rec2 := reader.RecordBatch()
	if rec2.NumRows() != 2 {
		t.Errorf("got %d rows, want 2", rec2.NumRows())
	}
}

func TestArrowStreamFormatter_Empty(t *testing.T) {
	var buf bytes.Buffer
	f := newArrowStreamFormatter(&buf)
	if err := f.Footer(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %d bytes", buf.Len())
	}
}

func TestArrowFileFormatter(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "x", Type: arrow.PrimitiveTypes.Int64},
	}, nil)
	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.Int64Builder).Append(42)
	})
	defer rec.Release()

	var buf bytes.Buffer
	f := newArrowFileFormatter(&buf)
	if err := f.Header(nil); err != nil {
		t.Fatal(err)
	}
	if err := f.Rows(rec); err != nil {
		t.Fatal(err)
	}
	if err := f.Footer(); err != nil {
		t.Fatal(err)
	}

	// Verify by reading back
	alloc := memory.NewGoAllocator()
	reader, err := ipc.NewFileReader(bytes.NewReader(buf.Bytes()), ipc.WithAllocator(alloc))
	if err != nil {
		t.Fatalf("invalid Arrow file: %v", err)
	}
	defer reader.Close() //nolint:errcheck

	if reader.NumRecords() != 1 {
		t.Errorf("got %d record batches, want 1", reader.NumRecords())
	}
}

func TestArrowFileFormatter_Empty(t *testing.T) {
	var buf bytes.Buffer
	f := newArrowFileFormatter(&buf)
	if err := f.Footer(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %d bytes", buf.Len())
	}
}
