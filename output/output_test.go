package output

import (
	"bytes"
	"encoding/json"
	"iter"
	"strings"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

// buildRecord creates an Arrow record from column names and row data (all string values).
func buildRecord(t *testing.T, columns []string, rows []map[string]interface{}) arrow.RecordBatch {
	t.Helper()
	fields := make([]arrow.Field, len(columns))
	for i, name := range columns {
		fields[i] = arrow.Field{Name: name, Type: arrow.BinaryTypes.String}
	}
	schema := arrow.NewSchema(fields, nil)
	alloc := memory.NewGoAllocator()
	b := array.NewRecordBuilder(alloc, schema)
	defer b.Release()
	for _, row := range rows {
		for i, col := range columns {
			b.Field(i).(*array.StringBuilder).Append(row[col].(string))
		}
	}
	return b.NewRecordBatch()
}

// staticResult is a test implementation of QueryResult.
type staticResult struct {
	records []arrow.RecordBatch
}

func (r *staticResult) StatementID() string { return "" }

func (r *staticResult) Chunks() iter.Seq2[arrow.RecordBatch, error] {
	return func(yield func(arrow.RecordBatch, error) bool) {
		for _, rec := range r.records {
			if !yield(rec, nil) {
				return
			}
		}
	}
}

func TestNewFormatter_UnsupportedFormat(t *testing.T) {
	var buf bytes.Buffer
	_, err := NewFormatter(&buf, "xml")
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestWriteResult_JSON(t *testing.T) {
	columns := []string{"name", "age"}
	rec1 := buildRecord(t, columns, []map[string]interface{}{{"name": "Alice", "age": "30"}})
	defer rec1.Release()
	rec2 := buildRecord(t, columns, []map[string]interface{}{{"name": "Bob", "age": "25"}})
	defer rec2.Release()

	result := &staticResult{records: []arrow.RecordBatch{rec1, rec2}}
	var buf bytes.Buffer
	n, err := WriteResult(&buf, result, "json")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("got %d rows, want 2", n)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var row map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
		t.Fatal(err)
	}
	if row["name"] != "Alice" {
		t.Errorf("first row name = %v, want Alice", row["name"])
	}
}

func TestWriteResult_CSV(t *testing.T) {
	columns := []string{"name", "age"}
	rec1 := buildRecord(t, columns, []map[string]interface{}{{"name": "Alice", "age": "30"}})
	defer rec1.Release()
	rec2 := buildRecord(t, columns, []map[string]interface{}{{"name": "Bob", "age": "25"}})
	defer rec2.Release()

	result := &staticResult{records: []arrow.RecordBatch{rec1, rec2}}
	var buf bytes.Buffer
	n, err := WriteResult(&buf, result, "csv")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("got %d rows, want 2", n)
	}
	expected := "name,age\nAlice,30\nBob,25\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestWriteResult_CSV_MultipleChunks(t *testing.T) {
	columns := []string{"name", "age"}
	rec1 := buildRecord(t, columns, []map[string]interface{}{
		{"name": "Alice", "age": "30"},
		{"name": "Bob", "age": "25"},
	})
	defer rec1.Release()
	rec2 := buildRecord(t, columns, []map[string]interface{}{
		{"name": "Charlie", "age": "35"},
	})
	defer rec2.Release()

	result := &staticResult{records: []arrow.RecordBatch{rec1, rec2}}
	var buf bytes.Buffer
	n, err := WriteResult(&buf, result, "csv")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("got %d rows, want 3", n)
	}
	expected := "name,age\nAlice,30\nBob,25\nCharlie,35\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestWriteResult_Empty(t *testing.T) {
	result := &staticResult{records: nil}
	var buf bytes.Buffer
	n, err := WriteResult(&buf, result, "json")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("got %d rows, want 0", n)
	}
	if buf.String() != "" {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}
