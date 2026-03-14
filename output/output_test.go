package output

import (
	"bytes"
	"encoding/json"
	"iter"
	"strings"
	"testing"
)

// staticResult is a test implementation of QueryResult.
type staticResult struct {
	columns []string
	chunks  [][]map[string]interface{}
}

func (r *staticResult) StatementID() string   { return "" }
func (r *staticResult) ColumnNames() []string { return r.columns }

func (r *staticResult) Chunks() iter.Seq2[[]map[string]interface{}, error] {
	return func(yield func([]map[string]interface{}, error) bool) {
		for _, chunk := range r.chunks {
			if !yield(chunk, nil) {
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
	result := &staticResult{
		columns: []string{"name", "age"},
		chunks: [][]map[string]interface{}{
			{{"name": "Alice", "age": "30"}},
			{{"name": "Bob", "age": "25"}},
		},
	}
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
	result := &staticResult{
		columns: []string{"name", "age"},
		chunks: [][]map[string]interface{}{
			{{"name": "Alice", "age": "30"}},
			{{"name": "Bob", "age": "25"}},
		},
	}
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
	result := &staticResult{
		columns: []string{"name", "age"},
		chunks: [][]map[string]interface{}{
			{
				{"name": "Alice", "age": "30"},
				{"name": "Bob", "age": "25"},
			},
			{
				{"name": "Charlie", "age": "35"},
			},
		},
	}
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
	result := &staticResult{
		columns: []string{"x"},
		chunks:  nil,
	}
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
