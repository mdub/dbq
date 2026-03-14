package output

import (
	"bytes"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
)

func TestCSVFormatter_Simple(t *testing.T) {
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
	f := newCSVFormatter(&buf)
	if err := f.Header([]string{"name", "age"}); err != nil {
		t.Fatal(err)
	}
	if err := f.Rows(rec); err != nil {
		t.Fatal(err)
	}
	if err := f.Footer(); err != nil {
		t.Fatal(err)
	}
	expected := "name,age\nAlice,30\nBob,25\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}

func TestCSVFormatter_QuotesCommas(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "description", Type: arrow.BinaryTypes.String},
		{Name: "value", Type: arrow.BinaryTypes.String},
	}, nil)
	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.StringBuilder).Append("has, comma")
		b.Field(1).(*array.StringBuilder).Append("ok")
		b.Field(0).(*array.StringBuilder).Append(`has "quotes"`)
		b.Field(1).(*array.StringBuilder).Append("ok")
		b.Field(0).(*array.StringBuilder).Append("has\nnewline")
		b.Field(1).(*array.StringBuilder).Append("ok")
	})
	defer rec.Release()

	var buf bytes.Buffer
	f := newCSVFormatter(&buf)
	if err := f.Header([]string{"description", "value"}); err != nil {
		t.Fatal(err)
	}
	if err := f.Rows(rec); err != nil {
		t.Fatal(err)
	}
	if err := f.Footer(); err != nil {
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
	if err := f.Header([]string{"x"}); err != nil {
		t.Fatal(err)
	}
	if err := f.Footer(); err != nil {
		t.Fatal(err)
	}
	expected := "x\n"
	if buf.String() != expected {
		t.Errorf("got %q, want %q", buf.String(), expected)
	}
}

func TestCSVFormatter_StructuredValues(t *testing.T) {
	structType := arrow.StructOf(
		arrow.Field{Name: "role", Type: arrow.BinaryTypes.String},
	)
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "name", Type: arrow.BinaryTypes.String},
		{Name: "tags", Type: arrow.ListOf(arrow.BinaryTypes.String)},
		{Name: "meta", Type: structType},
	}, nil)
	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.StringBuilder).Append("Alice")
		lb := b.Field(1).(*array.ListBuilder)
		lb.Append(true)
		lb.ValueBuilder().(*array.StringBuilder).Append("a")
		lb.ValueBuilder().(*array.StringBuilder).Append("b")
		sb := b.Field(2).(*array.StructBuilder)
		sb.Append(true)
		sb.FieldBuilder(0).(*array.StringBuilder).Append("admin")
	})
	defer rec.Release()

	var buf bytes.Buffer
	f := newCSVFormatter(&buf)
	if err := f.Header([]string{"name", "tags", "meta"}); err != nil {
		t.Fatal(err)
	}
	if err := f.Rows(rec); err != nil {
		t.Fatal(err)
	}
	if err := f.Footer(); err != nil {
		t.Fatal(err)
	}
	expected := "name,tags,meta\n" +
		"Alice,\"[\"\"a\"\",\"\"b\"\"]\",\"{\"\"role\"\":\"\"admin\"\"}\"\n"
	if buf.String() != expected {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), expected)
	}
}
