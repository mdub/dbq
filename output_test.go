package main

import (
	"bytes"
	"encoding/json"
	"iter"
	"strings"
	"testing"
)

// staticResult is a test implementation of QueryResult.
type staticResult struct {
	statementID string
	columns     []string
	chunks      [][]map[string]interface{}
}

func (r *staticResult) StatementID() string   { return r.statementID }
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

func TestCSVFormatter_Simple(t *testing.T) {
	columns := []string{"name", "age"}
	rows := []map[string]interface{}{
		{"name": "Alice", "age": "30"},
		{"name": "Bob", "age": "25"},
	}
	var buf bytes.Buffer
	f := &csvFormatter{w: &buf}
	if err := f.Start(columns); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteChunk(rows); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	expected := "name,age\nAlice,30\nBob,25\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestCSVFormatter_QuotesCommas(t *testing.T) {
	columns := []string{"description", "value"}
	rows := []map[string]interface{}{
		{"description": "has, comma", "value": "ok"},
		{"description": `has "quotes"`, "value": "ok"},
		{"description": "has\nnewline", "value": "ok"},
	}
	var buf bytes.Buffer
	f := &csvFormatter{w: &buf}
	if err := f.Start(columns); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteChunk(rows); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	expected := "description,value\n" +
		"\"has, comma\",ok\n" +
		"\"has \"\"quotes\"\"\",ok\n" +
		"\"has\nnewline\",ok\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestCSVFormatter_Empty(t *testing.T) {
	var buf bytes.Buffer
	f := &csvFormatter{w: &buf}
	if err := f.Start([]string{"x"}); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	expected := "x\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestCSVFormatter_StructuredValues(t *testing.T) {
	columns := []string{"name", "tags", "meta"}
	rows := []map[string]interface{}{
		{
			"name": "Alice",
			"tags": []interface{}{"a", "b"},
			"meta": map[string]interface{}{"role": "admin"},
		},
	}
	var buf bytes.Buffer
	f := &csvFormatter{w: &buf}
	if err := f.Start(columns); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteChunk(rows); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	expected := "name,tags,meta\n" +
		"Alice,\"[\"\"a\"\",\"\"b\"\"]\",\"{\"\"role\"\":\"\"admin\"\"}\"\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestJSONLFormatter(t *testing.T) {
	rows := []map[string]interface{}{
		{"name": "Alice", "age": "30"},
		{"name": "Bob", "age": "25"},
	}
	var buf bytes.Buffer
	f := &jsonlFormatter{enc: json.NewEncoder(&buf)}
	if err := f.Start([]string{"name", "age"}); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteChunk(rows); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}
	for i, line := range lines {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Fatalf("line %d: invalid JSON: %v", i, err)
		}
	}
}

func TestJSONLFormatter_Empty(t *testing.T) {
	var buf bytes.Buffer
	f := &jsonlFormatter{enc: json.NewEncoder(&buf)}
	if err := f.Start(nil); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "" {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

func TestJSONLFormatter_StructuredValues(t *testing.T) {
	rows := []map[string]interface{}{
		{
			"name": "Alice",
			"tags": []interface{}{"a", "b"},
			"meta": map[string]interface{}{"role": "admin"},
		},
	}
	var buf bytes.Buffer
	f := &jsonlFormatter{enc: json.NewEncoder(&buf)}
	if err := f.Start([]string{"name", "tags", "meta"}); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteChunk(rows); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
	tags, ok := parsed["tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Errorf("tags = %v, want [a b]", parsed["tags"])
	}
	meta, ok := parsed["meta"].(map[string]interface{})
	if !ok || meta["role"] != "admin" {
		t.Errorf("meta = %v, want {role: admin}", parsed["meta"])
	}
}

func TestNewResultFormatter_UnsupportedFormat(t *testing.T) {
	var buf bytes.Buffer
	_, err := newResultFormatter(&buf, "xml")
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
	n, err := writeResult(&buf, result, "json")
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
	n, err := writeResult(&buf, result, "csv")
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

func TestWriteResult_Empty(t *testing.T) {
	result := &staticResult{
		columns: []string{"x"},
		chunks:  nil,
	}
	var buf bytes.Buffer
	n, err := writeResult(&buf, result, "json")
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
