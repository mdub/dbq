package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"github.com/databricks/databricks-sdk-go/service/sql"
)

// columnMeta holds column name and type information
type columnMeta struct {
	name      string
	typeName  sql.ColumnInfoTypeName
	isComplex bool // true for STRUCT, MAP, ARRAY
}

// convertRows converts raw string arrays to typed maps
func convertRows(data [][]string, columns []columnMeta) []map[string]interface{} {
	var rows []map[string]interface{}
	for _, row := range data {
		rowMap := make(map[string]interface{})
		for i, val := range row {
			if i < len(columns) {
				col := columns[i]
				if col.isComplex && val != "" {
					// Try to parse complex types as JSON
					var parsed interface{}
					if err := json.Unmarshal([]byte(val), &parsed); err == nil {
						rowMap[col.name] = parsed
					} else {
						rowMap[col.name] = val
					}
				} else {
					rowMap[col.name] = val
				}
			}
		}
		rows = append(rows, rowMap)
	}
	return rows
}

// queryResult holds the processed results of a SQL query
type queryResult struct {
	columns []string
	rows    []map[string]interface{}
}

func (r *queryResult) writeCSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	if len(r.columns) > 0 {
		cw.Write(r.columns)
	}
	for _, row := range r.rows {
		record := make([]string, len(r.columns))
		for i, name := range r.columns {
			record[i] = fmt.Sprintf("%v", row[name])
		}
		cw.Write(record)
	}
	cw.Flush()
	return cw.Error()
}

func (r *queryResult) writeCSVRows(w io.Writer) error {
	cw := csv.NewWriter(w)
	for _, row := range r.rows {
		record := make([]string, len(r.columns))
		for i, name := range r.columns {
			record[i] = fmt.Sprintf("%v", row[name])
		}
		cw.Write(record)
	}
	cw.Flush()
	return cw.Error()
}

func (r *queryResult) writeJSONL(w io.Writer) error {
	enc := json.NewEncoder(w)
	for _, row := range r.rows {
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}
