package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/databricks/databricks-sdk-go/service/sql"
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

func TestConvertRows_SimpleStrings(t *testing.T) {
	columns := []columnMeta{
		{name: "name", typeName: sql.ColumnInfoTypeNameString},
		{name: "age", typeName: sql.ColumnInfoTypeNameInt},
	}
	data := [][]string{
		{"Alice", "30"},
		{"Bob", "25"},
	}
	rows := convertRows(data, columns)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0]["name"] != "Alice" || rows[0]["age"] != "30" {
		t.Errorf("row 0: %v", rows[0])
	}
	if rows[1]["name"] != "Bob" || rows[1]["age"] != "25" {
		t.Errorf("row 1: %v", rows[1])
	}
}

func TestConvertRows_ComplexTypes(t *testing.T) {
	columns := []columnMeta{
		{name: "data", typeName: sql.ColumnInfoTypeNameStruct, isComplex: true},
	}
	data := [][]string{
		{`{"key":"value"}`},
	}
	rows := convertRows(data, columns)
	m, ok := rows[0]["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected parsed map, got %T", rows[0]["data"])
	}
	if m["key"] != "value" {
		t.Errorf("got %v", m)
	}
}

func TestConvertRows_InvalidJSON(t *testing.T) {
	columns := []columnMeta{
		{name: "data", typeName: sql.ColumnInfoTypeNameStruct, isComplex: true},
	}
	data := [][]string{
		{"not json"},
	}
	rows := convertRows(data, columns)
	// Falls back to raw string
	if rows[0]["data"] != "not json" {
		t.Errorf("got %v", rows[0]["data"])
	}
}

func TestConvertRows_Empty(t *testing.T) {
	columns := []columnMeta{
		{name: "x", typeName: sql.ColumnInfoTypeNameString},
	}
	rows := convertRows(nil, columns)
	if len(rows) != 0 {
		t.Errorf("expected empty, got %d rows", len(rows))
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
