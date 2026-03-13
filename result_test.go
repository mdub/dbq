package main

import (
	"testing"

	"github.com/databricks/databricks-sdk-go/service/sql"
)

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
