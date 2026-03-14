package main

import (
	"fmt"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal128"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"io"
)

// readArrowStream reads an Arrow IPC stream and returns rows as typed maps.
func readArrowStream(r io.Reader) ([]string, []map[string]any, error) {
	reader, err := ipc.NewReader(r)
	if err != nil {
		return nil, nil, fmt.Errorf("reading Arrow stream: %w", err)
	}
	defer reader.Release()

	schema := reader.Schema()
	columnNames := make([]string, schema.NumFields())
	for i, f := range schema.Fields() {
		columnNames[i] = f.Name
	}

	var rows []map[string]any
	for reader.Next() {
		rec := reader.Record()
		rows = append(rows, recordToMaps(rec)...)
	}
	if err := reader.Err(); err != nil {
		return nil, nil, err
	}
	return columnNames, rows, nil
}

// recordToMaps converts an Arrow record batch to a slice of maps.
func recordToMaps(rec arrow.Record) []map[string]any {
	schema := rec.Schema()
	numRows := int(rec.NumRows())
	rows := make([]map[string]any, numRows)
	for i := range rows {
		rows[i] = make(map[string]any, rec.NumCols())
	}
	for colIdx := 0; colIdx < int(rec.NumCols()); colIdx++ {
		name := schema.Field(colIdx).Name
		col := rec.Column(colIdx)
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
		return int64(a.Value(i))
	case *array.Uint64:
		return int64(a.Value(i))
	case *array.Float32:
		return float64(a.Value(i))
	case *array.Float64:
		return a.Value(i)
	case *array.Decimal128:
		scale := a.DataType().(*arrow.Decimal128Type).Scale
		return decimal128ToFloat(a.Value(i), scale)
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
	case *array.Dictionary:
		dictIdx := a.GetValueIndex(i)
		return extractValue(a.Dictionary(), int(dictIdx))
	default:
		// Fallback: use Arrow's string representation
		return arr.ValueStr(i)
	}
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

func decimal128ToFloat(v decimal128.Num, scale int32) float64 {
	f := v.ToFloat64(scale)
	return f
}
