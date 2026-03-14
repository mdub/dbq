package output

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	iso8601 "github.com/Achsion/iso8601/v2"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
)

// ArrowToGo converts an Arrow record batch to a slice of maps.
func ArrowToGo(batch arrow.RecordBatch) []map[string]any {
	schema := batch.Schema()
	numRows := int(batch.NumRows())
	rows := make([]map[string]any, numRows)
	for i := range rows {
		rows[i] = make(map[string]any, batch.NumCols())
	}
	for colIdx := 0; colIdx < int(batch.NumCols()); colIdx++ {
		name := schema.Field(colIdx).Name
		col := batch.Column(colIdx)
		for rowIdx := 0; rowIdx < numRows; rowIdx++ {
			rows[rowIdx][name] = extractValue(col, rowIdx)
		}
	}
	return rows
}

// extractValue returns a typed Go value from an Arrow array at the given index.
func extractValue(arr arrow.Array, i int) any {
	if arr.IsNull(i) {
		return nil
	}
	switch a := arr.(type) {
	case *array.Boolean:
		return a.Value(i)
	case *array.Int8:
		return int64(a.Value(i))
	case *array.Int16:
		return int64(a.Value(i))
	case *array.Int32:
		return int64(a.Value(i))
	case *array.Int64:
		return a.Value(i)
	case *array.Uint8:
		return int64(a.Value(i))
	case *array.Uint16:
		return int64(a.Value(i))
	case *array.Uint32:
		return uint64(a.Value(i))
	case *array.Uint64:
		return a.Value(i)
	case *array.Float32:
		f64, _ := strconv.ParseFloat(strconv.FormatFloat(float64(a.Value(i)), 'f', -1, 32), 64)
		return f64
	case *array.Float64:
		return a.Value(i)
	case *array.Decimal128:
		typ := a.DataType().(*arrow.Decimal128Type)
		return json.Number(a.Value(i).ToBigFloat(typ.Scale).Text('f', int(typ.Scale)))
	case *array.String:
		return a.Value(i)
	case *array.LargeString:
		return a.Value(i)
	case *array.Binary:
		return a.Value(i)
	case *array.LargeBinary:
		return a.Value(i)
	case *array.Struct:
		return extractStruct(a, i)
	case *array.List:
		return extractList(a, i)
	case *array.LargeList:
		return extractLargeList(a, i)
	case *array.Map:
		return extractMap(a, i)
	case *array.Timestamp:
		return formatTimestamp(a, i)
	case *array.Date32:
		return a.Value(i).ToTime().Format(time.DateOnly)
	case *array.Date64:
		return a.Value(i).ToTime().Format(time.DateOnly)
	case *array.Duration:
		typ := a.DataType().(*arrow.DurationType)
		d := time.Duration(a.Value(i)) * durationUnit(typ.Unit)
		return iso8601.DurationFromTimeDuration(d).String()
	case *array.MonthInterval:
		return formatMonthInterval(int(a.Value(i)))
	case *array.Dictionary:
		dictIdx := a.GetValueIndex(i)
		return extractValue(a.Dictionary(), int(dictIdx))
	default:
		// Fallback: use Arrow's string representation
		return arr.ValueStr(i)
	}
}

func formatTimestamp(a *array.Timestamp, i int) string {
	typ := a.DataType().(*arrow.TimestampType)
	t := a.Value(i).ToTime(typ.Unit)
	layout := timestampLayout(typ.Unit)
	if typ.TimeZone != "" {
		loc, err := time.LoadLocation(typ.TimeZone)
		if err == nil {
			t = t.In(loc)
		}
		return t.Format(layout + "Z07:00")
	}
	return t.UTC().Format(layout)
}

func timestampLayout(unit arrow.TimeUnit) string {
	switch unit {
	case arrow.Second:
		return "2006-01-02T15:04:05"
	case arrow.Millisecond:
		return "2006-01-02T15:04:05.000"
	case arrow.Microsecond:
		return "2006-01-02T15:04:05.000000"
	case arrow.Nanosecond:
		return "2006-01-02T15:04:05.000000000"
	default:
		return "2006-01-02T15:04:05.999999999"
	}
}

func durationUnit(unit arrow.TimeUnit) time.Duration {
	switch unit {
	case arrow.Second:
		return time.Second
	case arrow.Millisecond:
		return time.Millisecond
	case arrow.Microsecond:
		return time.Microsecond
	case arrow.Nanosecond:
		return time.Nanosecond
	default:
		return time.Nanosecond
	}
}

func formatMonthInterval(months int) string {
	positive := months >= 0
	if !positive {
		months = -months
	}
	d, _ := iso8601.NewDuration(positive, float64(months/12), float64(months%12), 0, 0, 0, 0, 0)
	return d.String()
}

func extractStruct(a *array.Struct, i int) map[string]any {
	st := a.DataType().(*arrow.StructType)
	result := make(map[string]any, a.NumField())
	for f := 0; f < a.NumField(); f++ {
		result[st.Field(f).Name] = extractValue(a.Field(f), i)
	}
	return result
}

func extractList(a *array.List, i int) []any {
	start, end := a.ValueOffsets(i)
	values := a.ListValues()
	result := make([]any, 0, end-start)
	for j := start; j < end; j++ {
		result = append(result, extractValue(values, int(j)))
	}
	return result
}

func extractLargeList(a *array.LargeList, i int) []any {
	start, end := a.ValueOffsets(i)
	values := a.ListValues()
	result := make([]any, 0, end-start)
	for j := start; j < end; j++ {
		result = append(result, extractValue(values, int(j)))
	}
	return result
}

func extractMap(a *array.Map, i int) map[string]any {
	keys := a.Keys()
	items := a.Items()
	start, end := a.ValueOffsets(i)
	result := make(map[string]any, end-start)
	for j := start; j < end; j++ {
		key := fmt.Sprintf("%v", extractValue(keys, int(j)))
		result[key] = extractValue(items, int(j))
	}
	return result
}
