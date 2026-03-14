package output

import (
	"bytes"
	"testing"
)

func TestCSVFormatter_Simple(t *testing.T) {
	columns := []string{"name", "age"}
	rows := []map[string]interface{}{
		{"name": "Alice", "age": "30"},
		{"name": "Bob", "age": "25"},
	}
	var buf bytes.Buffer
	f := newCSVFormatter(&buf)
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
	f := newCSVFormatter(&buf)
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
	f := newCSVFormatter(&buf)
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
	f := newCSVFormatter(&buf)
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
