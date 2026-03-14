package output

import (
	"encoding/json"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal128"
	"github.com/apache/arrow-go/v18/arrow/memory"

	_ "time/tzdata"
)

func buildTestRecord(t *testing.T, schema *arrow.Schema, build func(*array.RecordBuilder)) arrow.RecordBatch {
	t.Helper()
	alloc := memory.NewGoAllocator()
	b := array.NewRecordBuilder(alloc, schema)
	defer b.Release()
	build(b)
	return b.NewRecordBatch()
}

func TestArrowToGo_IntegerTypes(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "i8", Type: arrow.PrimitiveTypes.Int8},
		{Name: "i16", Type: arrow.PrimitiveTypes.Int16},
		{Name: "i32", Type: arrow.PrimitiveTypes.Int32},
		{Name: "i64", Type: arrow.PrimitiveTypes.Int64},
		{Name: "u8", Type: arrow.PrimitiveTypes.Uint8},
		{Name: "u16", Type: arrow.PrimitiveTypes.Uint16},
		{Name: "u32", Type: arrow.PrimitiveTypes.Uint32},
		{Name: "u64", Type: arrow.PrimitiveTypes.Uint64},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.Int8Builder).Append(1)
		b.Field(1).(*array.Int16Builder).Append(2)
		b.Field(2).(*array.Int32Builder).Append(3)
		b.Field(3).(*array.Int64Builder).Append(4)
		b.Field(4).(*array.Uint8Builder).Append(5)
		b.Field(5).(*array.Uint16Builder).Append(6)
		b.Field(6).(*array.Uint32Builder).Append(7)
		b.Field(7).(*array.Uint64Builder).Append(8)
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	row := rows[0]
	if v := row["i8"]; v != int64(1) {
		t.Errorf("i8: got %v (%T)", v, v)
	}
	if v := row["i16"]; v != int64(2) {
		t.Errorf("i16: got %v (%T)", v, v)
	}
	if v := row["i32"]; v != int64(3) {
		t.Errorf("i32: got %v (%T)", v, v)
	}
	if v := row["i64"]; v != int64(4) {
		t.Errorf("i64: got %v (%T)", v, v)
	}
	if v := row["u8"]; v != int64(5) {
		t.Errorf("u8: got %v (%T)", v, v)
	}
	if v := row["u16"]; v != int64(6) {
		t.Errorf("u16: got %v (%T)", v, v)
	}
	if v := row["u32"]; v != uint64(7) {
		t.Errorf("u32: got %v (%T)", v, v)
	}
	if v := row["u64"]; v != uint64(8) {
		t.Errorf("u64: got %v (%T)", v, v)
	}
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

func TestArrowToGo_Binary(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "b", Type: arrow.BinaryTypes.Binary},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.BinaryBuilder).Append([]byte{0x01, 0x02, 0x03})
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	got, ok := rows[0]["b"].([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", rows[0]["b"])
	}
	if len(got) != 3 || got[0] != 0x01 {
		t.Errorf("got %v", got)
	}
}

func TestArrowToGo_LargeString(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "s", Type: arrow.BinaryTypes.LargeString},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.LargeStringBuilder).Append("hello")
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	if rows[0]["s"] != "hello" {
		t.Errorf("got %v", rows[0]["s"])
	}
}

func TestArrowToGo_Map(t *testing.T) {
	mapType := arrow.MapOf(arrow.BinaryTypes.String, arrow.PrimitiveTypes.Int64)
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "m", Type: mapType},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		mb := b.Field(0).(*array.MapBuilder)
		kb := mb.KeyBuilder().(*array.StringBuilder)
		vb := mb.ItemBuilder().(*array.Int64Builder)
		mb.Append(true)
		kb.Append("a")
		vb.Append(1)
		kb.Append("b")
		vb.Append(2)
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	m, ok := rows[0]["m"].(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", rows[0]["m"])
	}
	if m["a"] != int64(1) || m["b"] != int64(2) {
		t.Errorf("got %v", m)
	}
}

func TestArrowToGo_Dictionary(t *testing.T) {
	dictType := &arrow.DictionaryType{
		IndexType: arrow.PrimitiveTypes.Int32,
		ValueType: arrow.BinaryTypes.String,
	}
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "d", Type: dictType},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		db := b.Field(0).(*array.BinaryDictionaryBuilder)
		_ = db.AppendString("cat")
		_ = db.AppendString("dog")
		_ = db.AppendString("cat")
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	if rows[0]["d"] != "cat" {
		t.Errorf("row 0: got %v", rows[0]["d"])
	}
	if rows[1]["d"] != "dog" {
		t.Errorf("row 1: got %v", rows[1]["d"])
	}
	if rows[2]["d"] != "cat" {
		t.Errorf("row 2: got %v", rows[2]["d"])
	}
}

func TestArrowToGo_TimestampSecond(t *testing.T) {
	tsType := &arrow.TimestampType{Unit: arrow.Second, TimeZone: "Etc/UTC"}
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "ts", Type: tsType},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.TimestampBuilder).Append(arrow.Timestamp(1773477000))
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	got := rows[0]["ts"].(string)
	want := "2026-03-14T08:30:00Z"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArrowToGo_TimestampNanosecond(t *testing.T) {
	tsType := &arrow.TimestampType{Unit: arrow.Nanosecond, TimeZone: "Etc/UTC"}
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "ts", Type: tsType},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.TimestampBuilder).Append(arrow.Timestamp(1773477000123456789))
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	got := rows[0]["ts"].(string)
	want := "2026-03-14T08:30:00.123456789Z"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArrowToGo_DurationSecond(t *testing.T) {
	durType := &arrow.DurationType{Unit: arrow.Second}
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "d", Type: durType},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.DurationBuilder).Append(arrow.Duration(90))
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	got := rows[0]["d"].(string)
	want := "PT1M30S"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArrowToGo_DurationMillisecond(t *testing.T) {
	durType := &arrow.DurationType{Unit: arrow.Millisecond}
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "d", Type: durType},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.DurationBuilder).Append(arrow.Duration(1500))
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	got := rows[0]["d"].(string)
	want := "PT1.5S"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArrowToGo_DurationNanosecond(t *testing.T) {
	durType := &arrow.DurationType{Unit: arrow.Nanosecond}
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "d", Type: durType},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.DurationBuilder).Append(arrow.Duration(1_000_000_000))
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	got := rows[0]["d"].(string)
	want := "PT1S"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArrowToGo_Decimal128(t *testing.T) {
	// Decimal128 with precision=38, scale=10
	decType := &arrow.Decimal128Type{Precision: 38, Scale: 10}
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "amount", Type: decType},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.Decimal128Builder).Append(decimal128.FromI64(12345678901234))
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	val := rows[0]["amount"]
	num, ok := val.(json.Number)
	if !ok {
		t.Fatalf("expected json.Number, got %T (%v)", val, val)
	}
	// 12345678901234 with scale=10 means 1234.5678901234
	if num.String() != "1234.5678901234" {
		t.Errorf("got %s, want 1234.5678901234", num.String())
	}
}

func TestArrowToGo_TimestampWithTZ(t *testing.T) {
	tsType := &arrow.TimestampType{Unit: arrow.Microsecond, TimeZone: "Etc/UTC"}
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "ts", Type: tsType},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		// 2026-03-14T08:30:00.123456Z in microseconds since epoch
		b.Field(0).(*array.TimestampBuilder).Append(arrow.Timestamp(1773477000123456))
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	got := rows[0]["ts"].(string)
	want := "2026-03-14T08:30:00.123456Z"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArrowToGo_TimestampWithNonUTCTZ(t *testing.T) {
	tsType := &arrow.TimestampType{Unit: arrow.Microsecond, TimeZone: "Australia/Melbourne"}
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "ts", Type: tsType},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		// 2026-03-14T08:30:00Z UTC = 2026-03-14T19:30:00+11:00 AEDT
		b.Field(0).(*array.TimestampBuilder).Append(arrow.Timestamp(1773477000000000))
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	got := rows[0]["ts"].(string)
	want := "2026-03-14T19:30:00.000000+11:00"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArrowToGo_TimestampNTZ(t *testing.T) {
	tsType := &arrow.TimestampType{Unit: arrow.Microsecond}
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "ts_ntz", Type: tsType},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.TimestampBuilder).Append(arrow.Timestamp(1773477000123456))
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	got := rows[0]["ts_ntz"].(string)
	want := "2026-03-14T08:30:00.123456"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArrowToGo_Date(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "d", Type: arrow.FixedWidthTypes.Date32},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		// 2026-03-14 = 20527 days since epoch
		b.Field(0).(*array.Date32Builder).Append(arrow.Date32(20526))
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	got := rows[0]["d"].(string)
	want := "2026-03-14"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArrowToGo_TimestampMillisecond(t *testing.T) {
	tsType := &arrow.TimestampType{Unit: arrow.Millisecond, TimeZone: "Etc/UTC"}
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "ts", Type: tsType},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		// 2026-03-14T08:30:00.123Z in milliseconds since epoch
		b.Field(0).(*array.TimestampBuilder).Append(arrow.Timestamp(1773477000123))
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	got := rows[0]["ts"].(string)
	want := "2026-03-14T08:30:00.123Z"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArrowToGo_Float32Precision(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "f", Type: arrow.PrimitiveTypes.Float32},
	}, nil)

	rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
		b.Field(0).(*array.Float32Builder).Append(1.1)
		b.Field(0).(*array.Float32Builder).Append(0.1)
		b.Field(0).(*array.Float32Builder).Append(123456.789)
	})
	defer rec.Release()

	rows := ArrowToGo(rec)
	tests := []struct {
		row  int
		want float64
	}{
		{0, 1.1},
		{1, 0.1},
		{2, 123456.79},
	}
	for _, tt := range tests {
		got := rows[tt.row]["f"].(float64)
		if got != tt.want {
			t.Errorf("row %d: got %v, want %v", tt.row, got, tt.want)
		}
	}
}

func TestArrowToGo_Duration(t *testing.T) {
	durType := &arrow.DurationType{Unit: arrow.Microsecond}
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "d", Type: durType},
	}, nil)

	tests := []struct {
		microseconds int64
		want         string
	}{
		{0, "PT0S"},
		{1_000_000, "PT1S"},
		{3_600_000_000, "PT1H"},
		{5_400_000_000, "PT1H30M"},
		{93_784_000_000, "PT26H3M4S"},
		{1_500_000, "PT1.5S"},
		{123_456, "PT0.123456S"},
		{-3_600_000_000, "-PT1H"},
	}
	for _, tt := range tests {
		rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
			b.Field(0).(*array.DurationBuilder).Append(arrow.Duration(tt.microseconds))
		})
		rows := ArrowToGo(rec)
		got := rows[0]["d"].(string)
		rec.Release()
		if got != tt.want {
			t.Errorf("Duration(%d us): got %q, want %q", tt.microseconds, got, tt.want)
		}
	}
}

func TestArrowToGo_MonthInterval(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "m", Type: arrow.FixedWidthTypes.MonthInterval},
	}, nil)

	tests := []struct {
		months int32
		want   string
	}{
		{0, "PT0S"},
		{1, "P1M"},
		{12, "P1Y"},
		{14, "P1Y2M"},
		{-3, "-P3M"},
		{-14, "-P1Y2M"},
	}
	for _, tt := range tests {
		rec := buildTestRecord(t, schema, func(b *array.RecordBuilder) {
			b.Field(0).(*array.MonthIntervalBuilder).Append(arrow.MonthInterval(tt.months))
		})
		rows := ArrowToGo(rec)
		got := rows[0]["m"].(string)
		rec.Release()
		if got != tt.want {
			t.Errorf("MonthInterval(%d): got %q, want %q", tt.months, got, tt.want)
		}
	}
}
