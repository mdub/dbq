package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/mdub/dbq/result"
	"iter"
)

// mockResult implements result.QueryResult for testing.
type mockResult struct {
	columns []string
	batches []arrow.RecordBatch
}

func (r *mockResult) StatementID() string   { return "test-id" }
func (r *mockResult) ColumnNames() []string { return r.columns }
func (r *mockResult) Chunks() iter.Seq2[arrow.RecordBatch, error] {
	return func(yield func(arrow.RecordBatch, error) bool) {
		for _, b := range r.batches {
			if !yield(b, nil) {
				return
			}
		}
	}
}

var _ result.QueryResult = (*mockResult)(nil)

func buildBatch(t *testing.T) arrow.RecordBatch {
	t.Helper()
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "x", Type: arrow.PrimitiveTypes.Int64},
	}, nil)
	alloc := memory.NewGoAllocator()
	b := array.NewRecordBuilder(alloc, schema)
	defer b.Release()
	b.Field(0).(*array.Int64Builder).Append(42)
	return b.NewRecordBatch()
}

func TestResolveFormat_Default(t *testing.T) {
	o := OutputOptions{}
	f, err := o.resolveFormat()
	if err != nil {
		t.Fatal(err)
	}
	if f != "jsonl" {
		t.Errorf("got %q, want %q", f, "jsonl")
	}
}

func TestResolveFormat_ExplicitFormat(t *testing.T) {
	o := OutputOptions{Format: "csv"}
	f, err := o.resolveFormat()
	if err != nil {
		t.Fatal(err)
	}
	if f != "csv" {
		t.Errorf("got %q, want %q", f, "csv")
	}
}

func TestResolveFormat_FromExtension(t *testing.T) {
	tests := []struct {
		output string
		want   string
	}{
		{"out.csv", "csv"},
		{"out.json", "json"},
		{"out.jsonl", "jsonl"},
		{"out.parquet", "parquet"},
		{"OUT.CSV", "csv"},
	}
	for _, tt := range tests {
		o := OutputOptions{Output: tt.output}
		f, err := o.resolveFormat()
		if err != nil {
			t.Errorf("Output=%q: %v", tt.output, err)
			continue
		}
		if f != tt.want {
			t.Errorf("Output=%q: got %q, want %q", tt.output, f, tt.want)
		}
	}
}

func TestResolveFormat_MutuallyExclusive(t *testing.T) {
	o := OutputOptions{Format: "csv", Output: "out.json"}
	_, err := o.resolveFormat()
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags")
	}
}

func TestResolveFormat_NoExtension(t *testing.T) {
	o := OutputOptions{Output: "outfile"}
	_, err := o.resolveFormat()
	if err == nil {
		t.Fatal("expected error for missing extension")
	}
}

func TestWriteResult_ToFile(t *testing.T) {
	batch := buildBatch(t)
	defer batch.Release()

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.jsonl")

	o := OutputOptions{Output: outPath}
	qr := &mockResult{
		columns: []string{"x"},
		batches: []arrow.RecordBatch{batch},
	}
	n, err := o.WriteResult(qr)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("got %d rows, want 1", n)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty output file")
	}
}

func TestWriteResult_ToParquetFile(t *testing.T) {
	batch := buildBatch(t)
	defer batch.Release()

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.parquet")

	o := OutputOptions{Output: outPath}
	qr := &mockResult{
		columns: []string{"x"},
		batches: []arrow.RecordBatch{batch},
	}
	n, err := o.WriteResult(qr)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("got %d rows, want 1", n)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Error("expected non-empty parquet file")
	}
}
