package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
)

func TestJSONFormatter(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "name", Type: arrow.BinaryTypes.String},
		{Name: "age", Type: arrow.BinaryTypes.String},
	}, nil)
	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.StringBuilder).Append("Alice")
		b.Field(1).(*array.StringBuilder).Append("30")
		b.Field(0).(*array.StringBuilder).Append("Bob")
		b.Field(1).(*array.StringBuilder).Append("25")
	})
	defer rec.Release()

	var buf bytes.Buffer
	f := newJSONFormatter(&buf)
	if err := f.Rows(rec); err != nil {
		t.Fatal(err)
	}
	if err := f.Footer(); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "Alice" {
		t.Errorf("first row name = %v, want Alice", rows[0]["name"])
	}
	if rows[1]["name"] != "Bob" {
		t.Errorf("second row name = %v, want Bob", rows[1]["name"])
	}
}

func TestJSONFormatter_MultipleChunks(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "x", Type: arrow.BinaryTypes.String},
	}, nil)
	rec1 := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.StringBuilder).Append("a")
	})
	defer rec1.Release()
	rec2 := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.StringBuilder).Append("b")
		b.Field(0).(*array.StringBuilder).Append("c")
	})
	defer rec2.Release()

	var buf bytes.Buffer
	f := newJSONFormatter(&buf)
	if err := f.Rows(rec1); err != nil {
		t.Fatal(err)
	}
	if err := f.Rows(rec2); err != nil {
		t.Fatal(err)
	}
	if err := f.Footer(); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0]["x"] != "a" || rows[1]["x"] != "b" || rows[2]["x"] != "c" {
		t.Errorf("got %v, want [a, b, c]", rows)
	}
}

func TestJSONFormatter_Empty(t *testing.T) {
	var buf bytes.Buffer
	f := newJSONFormatter(&buf)
	if err := f.Footer(); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "[]\n" {
		t.Errorf("expected empty array, got %q", buf.String())
	}
}
