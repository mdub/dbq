package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
)

// writeResult writes query results to w in the specified format.
func writeResult(w io.Writer, result QueryResult, format string) (int, error) {
	columnNames := result.ColumnNames()

	// "raw" format needs all rows buffered
	if format == "raw" {
		var allRows []map[string]interface{}
		for chunk, err := range result.Chunks() {
			if err != nil {
				return 0, err
			}
			allRows = append(allRows, chunk...)
		}
		output := map[string]interface{}{
			"statement_id": result.StatementID(),
			"columns":      columnNames,
			"rows":         allRows,
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return len(allRows), enc.Encode(output)
	}

	// Stream rows chunk-by-chunk for json and csv formats
	rowCount := 0
	writeChunk := newChunkWriter(w, columnNames, format)
	for chunk, err := range result.Chunks() {
		if err != nil {
			return 0, err
		}
		if err := writeChunk(chunk); err != nil {
			return 0, err
		}
		rowCount += len(chunk)
	}
	return rowCount, nil
}

// newChunkWriter returns a function that writes a batch of rows in the selected format.
func newChunkWriter(w io.Writer, columnNames []string, format string) func([]map[string]interface{}) error {
	headerWritten := false
	return func(rows []map[string]interface{}) error {
		switch format {
		case "csv":
			if !headerWritten {
				if err := writeCSV(w, columnNames, rows); err != nil {
					return fmt.Errorf("CSV write error: %w", err)
				}
				headerWritten = true
			} else {
				if err := writeCSVRows(w, columnNames, rows); err != nil {
					return fmt.Errorf("CSV write error: %w", err)
				}
			}
		default:
			if err := writeJSONL(w, rows); err != nil {
				return fmt.Errorf("JSONL write error: %w", err)
			}
		}
		return nil
	}
}

func writeCSV(w io.Writer, columns []string, rows []map[string]interface{}) error {
	cw := csv.NewWriter(w)
	if len(columns) > 0 {
		cw.Write(columns)
	}
	for _, row := range rows {
		record := make([]string, len(columns))
		for i, name := range columns {
			record[i] = fmt.Sprintf("%v", row[name])
		}
		cw.Write(record)
	}
	cw.Flush()
	return cw.Error()
}

func writeCSVRows(w io.Writer, columns []string, rows []map[string]interface{}) error {
	cw := csv.NewWriter(w)
	for _, row := range rows {
		record := make([]string, len(columns))
		for i, name := range columns {
			record[i] = fmt.Sprintf("%v", row[name])
		}
		cw.Write(record)
	}
	cw.Flush()
	return cw.Error()
}

func writeJSONL(w io.Writer, rows []map[string]interface{}) error {
	enc := json.NewEncoder(w)
	for _, row := range rows {
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}
