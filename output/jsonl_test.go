package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
)

func TestJSONLFormatter(t *testing.T) {
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
	f := newJSONLFormatter(&buf)
	if err := f.WriteRecordBatch(rec); err != nil {
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
	f := newJSONLFormatter(&buf)
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "" {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

func TestJSONLFormatter_StructuredValues(t *testing.T) {
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
	f := newJSONLFormatter(&buf)
	if err := f.WriteRecordBatch(rec); err != nil {
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
