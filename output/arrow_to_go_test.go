package output

import (
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

func buildTestRecord(t *testing.T, schema *arrow.Schema, build func(*array.RecordBuilder)) arrow.RecordBatch {
	t.Helper()
	alloc := memory.NewGoAllocator()
	b := array.NewRecordBuilder(alloc, schema)
	defer b.Release()
	build(b)
	return b.NewRecordBatch()
}

func TestArrowToGo_ScalarTypes(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "name", Type: arrow.BinaryTypes.String},
		{Name: "age", Type: arrow.PrimitiveTypes.Int64},
		{Name: "score", Type: arrow.PrimitiveTypes.Float64},
		{Name: "active", Type: arrow.FixedWidthTypes.Boolean},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.StringBuilder).Append("Alice")
		b.Field(1).(*array.Int64Builder).Append(30)
		b.Field(2).(*array.Float64Builder).Append(9.5)
		b.Field(3).(*array.BooleanBuilder).Append(true)

		b.Field(0).(*array.StringBuilder).Append("Bob")
		b.Field(1).(*array.Int64Builder).Append(25)
		b.Field(2).(*array.Float64Builder).Append(8.0)
		b.Field(3).(*array.BooleanBuilder).Append(false)
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0]["name"] != "Alice" {
		t.Errorf("name: got %v", rows[0]["name"])
	}
	if rows[0]["age"] != int64(30) {
		t.Errorf("age: got %v (%T)", rows[0]["age"], rows[0]["age"])
	}
	if rows[0]["score"] != 9.5 {
		t.Errorf("score: got %v", rows[0]["score"])
	}
	if rows[0]["active"] != true {
		t.Errorf("active: got %v", rows[0]["active"])
	}
	if rows[1]["active"] != false {
		t.Errorf("active: got %v", rows[1]["active"])
	}
}

func TestArrowToGo_Nulls(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "age", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.StringBuilder).Append("Alice")
		b.Field(1).(*array.Int64Builder).AppendNull()

		b.Field(0).(*array.StringBuilder).AppendNull()
		b.Field(1).(*array.Int64Builder).Append(25)
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	if rows[0]["age"] != nil {
		t.Errorf("expected nil age, got %v", rows[0]["age"])
	}
	if rows[1]["name"] != nil {
		t.Errorf("expected nil name, got %v", rows[1]["name"])
	}
}

func TestArrowToGo_Struct(t *testing.T) {
	structType := arrow.StructOf(
		arrow.Field{Name: "x", Type: arrow.PrimitiveTypes.Int64},
		arrow.Field{Name: "y", Type: arrow.BinaryTypes.String},
	)
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "data", Type: structType},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		sb := b.Field(0).(*array.StructBuilder)
		sb.Append(true)
		sb.FieldBuilder(0).(*array.Int64Builder).Append(42)
		sb.FieldBuilder(1).(*array.StringBuilder).Append("hello")
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	m, ok := rows[0]["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", rows[0]["data"])
	}
	if m["x"] != int64(42) {
		t.Errorf("x: got %v (%T)", m["x"], m["x"])
	}
	if m["y"] != "hello" {
		t.Errorf("y: got %v", m["y"])
	}
}

func TestArrowToGo_List(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "tags", Type: arrow.ListOf(arrow.BinaryTypes.String)},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		lb := b.Field(0).(*array.ListBuilder)
		vb := lb.ValueBuilder().(*array.StringBuilder)
		lb.Append(true)
		vb.Append("a")
		vb.Append("b")
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	list, ok := rows[0]["tags"].([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", rows[0]["tags"])
	}
	if len(list) != 2 || list[0] != "a" || list[1] != "b" {
		t.Errorf("got %v", list)
	}
}
