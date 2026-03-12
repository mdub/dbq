package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWriteCSV_Simple(t *testing.T) {
	r := &queryResult{
		columns: []string{"name", "age"},
		rows: []map[string]interface{}{
			{"name": "Alice", "age": "30"},
			{"name": "Bob", "age": "25"},
		},
	}
	var buf bytes.Buffer
	if err := r.writeCSV(&buf); err != nil {
		t.Fatal(err)
	}
	expected := "name,age\nAlice,30\nBob,25\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestWriteCSV_QuotesCommas(t *testing.T) {
	r := &queryResult{
		columns: []string{"description", "value"},
		rows: []map[string]interface{}{
			{"description": "has, comma", "value": "ok"},
			{"description": `has "quotes"`, "value": "ok"},
			{"description": "has\nnewline", "value": "ok"},
		},
	}
	var buf bytes.Buffer
	if err := r.writeCSV(&buf); err != nil {
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

func TestWriteCSV_Empty(t *testing.T) {
	r := &queryResult{
		columns: []string{"x"},
		rows:    nil,
	}
	var buf bytes.Buffer
	if err := r.writeCSV(&buf); err != nil {
		t.Fatal(err)
	}
	expected := "x\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestWriteJSON(t *testing.T) {
	r := &queryResult{
		columns: []string{"name"},
		rows: []map[string]interface{}{
			{"name": "Alice"},
		},
	}
	var buf bytes.Buffer
	if err := r.writeJSON(&buf); err != nil {
		t.Fatal(err)
	}
	var parsed []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed) != 1 || parsed[0]["name"] != "Alice" {
		t.Errorf("unexpected JSON: %s", buf.String())
	}
}
