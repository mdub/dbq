package output

import (
	"bytes"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet/file"
)

func TestParquetFormatter_ScalarTypes(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "n", Type: arrow.PrimitiveTypes.Int64},
		{Name: "d", Type: arrow.PrimitiveTypes.Float64},
		{Name: "b", Type: arrow.FixedWidthTypes.Boolean},
		{Name: "s", Type: arrow.BinaryTypes.String},
	}, nil)

	batch := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.Int64Builder).Append(42)
		b.Field(1).(*array.Float64Builder).Append(3.14)
		b.Field(2).(*array.BooleanBuilder).Append(true)
		b.Field(3).(*array.StringBuilder).Append("hello")
	})
	defer batch.Release()

	var buf bytes.Buffer
	f := newParquetFormatter(&buf)
	if err := f.Rows(batch); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	reader, err := file.NewParquetReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("invalid Parquet output: %v", err)
	}
	defer reader.Close() //nolint:errcheck

	if reader.NumRows() != 1 {
		t.Errorf("got %d rows, want 1", reader.NumRows())
	}
	if reader.MetaData().Schema.NumColumns() != 4 {
		t.Errorf("got %d columns, want 4", reader.MetaData().Schema.NumColumns())
	}
}

func TestParquetFormatter_MultipleBatches(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "x", Type: arrow.PrimitiveTypes.Int64},
	}, nil)

	alloc := memory.NewGoAllocator()
	b := array.NewRecordBuilder(alloc, schema)
	defer b.Release()

	b.Field(0).(*array.Int64Builder).Append(1)
	batch1 := b.NewRecordBatch()
	defer batch1.Release()

	b.Field(0).(*array.Int64Builder).Append(2)
	b.Field(0).(*array.Int64Builder).Append(3)
	batch2 := b.NewRecordBatch()
	defer batch2.Release()

	var buf bytes.Buffer
	f := newParquetFormatter(&buf)
	if err := f.Rows(batch1); err != nil {
		t.Fatal(err)
	}
	if err := f.Rows(batch2); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	reader, err := file.NewParquetReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("invalid Parquet output: %v", err)
	}
	defer reader.Close() //nolint:errcheck

	if reader.NumRows() != 3 {
		t.Errorf("got %d rows, want 3", reader.NumRows())
	}
}

func TestParquetFormatter_RoundTrip(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "name", Type: arrow.BinaryTypes.String},
		{Name: "value", Type: arrow.PrimitiveTypes.Int64},
	}, nil)

	batch := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.StringBuilder).Append("Alice")
		b.Field(1).(*array.Int64Builder).Append(42)
		b.Field(0).(*array.StringBuilder).Append("Bob")
		b.Field(1).(*array.Int64Builder).Append(99)
	})
	defer batch.Release()

	var buf bytes.Buffer
	f := newParquetFormatter(&buf)
	if err := f.Rows(batch); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	reader, err := file.NewParquetReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close() //nolint:errcheck

	if reader.NumRows() != 2 {
		t.Errorf("got %d rows, want 2", reader.NumRows())
	}
	pqSchema := reader.MetaData().Schema
	if pqSchema.NumColumns() != 2 {
		t.Errorf("got %d columns, want 2", pqSchema.NumColumns())
	}
	if pqSchema.Column(0).Name() != "name" {
		t.Errorf("column 0 name: got %q, want %q", pqSchema.Column(0).Name(), "name")
	}
	if pqSchema.Column(1).Name() != "value" {
		t.Errorf("column 1 name: got %q, want %q", pqSchema.Column(1).Name(), "value")
	}
}

func TestParquetFormatter_CloseWithoutWrite(t *testing.T) {
	var buf bytes.Buffer
	f := newParquetFormatter(&buf)
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %d bytes", buf.Len())
	}
}
