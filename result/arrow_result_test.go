package result

import (
	"bytes"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/databricks/databricks-sdk-go/service/sql"
)

// buildArrowStream creates an Arrow IPC stream from a schema and a builder function.
func buildArrowStream(t *testing.T, schema *arrow.Schema, build func(*array.RecordBuilder)) []byte {
	t.Helper()
	alloc := memory.NewGoAllocator()
	b := array.NewRecordBuilder(alloc, schema)
	defer b.Release()
	build(b)
	rec := b.NewRecordBatch()
	defer rec.Release()

	var buf bytes.Buffer
	w := ipc.NewWriter(&buf, ipc.WithSchema(schema))
	if err := w.Write(rec); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestStatementID(t *testing.T) {
	r := &arrowResult{
		response: &sql.StatementResponse{StatementId: "abc-123"},
	}
	if got := r.StatementID(); got != "abc-123" {
		t.Errorf("got %q, want %q", got, "abc-123")
	}
}

func TestColumnNames(t *testing.T) {
	r := &arrowResult{
		response: &sql.StatementResponse{
			Manifest: &sql.ResultManifest{
				Schema: &sql.ResultSchema{
					Columns: []sql.ColumnInfo{
						{Name: "name"},
						{Name: "age"},
					},
				},
			},
		},
	}
	names := r.ColumnNames()
	if len(names) != 2 || names[0] != "name" || names[1] != "age" {
		t.Errorf("got %v", names)
	}
}

func TestColumnNames_NilManifest(t *testing.T) {
	r := &arrowResult{
		response: &sql.StatementResponse{},
	}
	if names := r.ColumnNames(); names != nil {
		t.Errorf("expected nil, got %v", names)
	}
}

func TestColumnNames_NilSchema(t *testing.T) {
	r := &arrowResult{
		response: &sql.StatementResponse{
			Manifest: &sql.ResultManifest{},
		},
	}
	if names := r.ColumnNames(); names != nil {
		t.Errorf("expected nil, got %v", names)
	}
}

func TestReadArrowStream_InvalidData(t *testing.T) {
	var gotErr error
	for _, err := range readArrowStream(bytes.NewReader([]byte("not arrow data"))) {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr == nil {
		t.Fatal("expected error for invalid Arrow data")
	}
}

func TestReadArrowStream_MultipleBatches(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "x", Type: arrow.PrimitiveTypes.Int64},
	}, nil)

	alloc := memory.NewGoAllocator()
	b := array.NewRecordBuilder(alloc, schema)
	defer b.Release()

	b.Field(0).(*array.Int64Builder).Append(1)
	rec1 := b.NewRecordBatch()
	defer rec1.Release()

	b.Field(0).(*array.Int64Builder).Append(2)
	b.Field(0).(*array.Int64Builder).Append(3)
	rec2 := b.NewRecordBatch()
	defer rec2.Release()

	var buf bytes.Buffer
	w := ipc.NewWriter(&buf, ipc.WithSchema(schema))
	if err := w.Write(rec1); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(rec2); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	var totalRows int64
	var batchCount int
	for rec, err := range readArrowStream(bytes.NewReader(buf.Bytes())) {
		if err != nil {
			t.Fatal(err)
		}
		totalRows += rec.NumRows()
		batchCount++
	}
	if batchCount != 2 {
		t.Errorf("got %d batches, want 2", batchCount)
	}
	if totalRows != 3 {
		t.Errorf("got %d total rows, want 3", totalRows)
	}
}

func TestReadArrowStream_ScalarTypes(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "name", Type: arrow.BinaryTypes.String},
		{Name: "age", Type: arrow.PrimitiveTypes.Int64},
		{Name: "score", Type: arrow.PrimitiveTypes.Float64},
		{Name: "active", Type: arrow.FixedWidthTypes.Boolean},
	}, nil)

	data := buildArrowStream(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.StringBuilder).Append("Alice")
		b.Field(1).(*array.Int64Builder).Append(30)
		b.Field(2).(*array.Float64Builder).Append(9.5)
		b.Field(3).(*array.BooleanBuilder).Append(true)

		b.Field(0).(*array.StringBuilder).Append("Bob")
		b.Field(1).(*array.Int64Builder).Append(25)
		b.Field(2).(*array.Float64Builder).Append(8.0)
		b.Field(3).(*array.BooleanBuilder).Append(false)
	})

	var records []arrow.RecordBatch
	for rec, err := range readArrowStream(bytes.NewReader(data)) {
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, rec)
	}
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}

	rec := records[0]
	if rec.Schema().NumFields() != 4 {
		t.Fatalf("got %d columns, want 4", rec.Schema().NumFields())
	}
	if rec.NumRows() != 2 {
		t.Fatalf("got %d rows, want 2", rec.NumRows())
	}
}

func TestReadArrowStream_Empty(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "x", Type: arrow.PrimitiveTypes.Int64},
	}, nil)

	data := buildArrowStream(t, schema, func(b *array.RecordBuilder) {
		// no rows
	})

	var records []arrow.RecordBatch
	for rec, err := range readArrowStream(bytes.NewReader(data)) {
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, rec)
	}
	// Empty stream may yield a record with 0 rows or no records
	for _, rec := range records {
		if rec.NumRows() != 0 {
			t.Errorf("expected 0 rows, got %d", rec.NumRows())
		}
	}
}
