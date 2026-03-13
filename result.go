package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/databricks/databricks-sdk-go"
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

// outputResult writes the query results to stdout in the specified format.
func outputResult(ctx context.Context, client *databricks.WorkspaceClient, response *sql.StatementResponse, format string) (int, error) {
	if response.Status.State == sql.StatementStateFailed {
		return 0, fmt.Errorf("query failed: %s", response.Status.Error.Message)
	}

	var columns []columnMeta
	if response.Manifest != nil && response.Manifest.Schema != nil {
		for _, col := range response.Manifest.Schema.Columns {
			isComplex := col.TypeName == sql.ColumnInfoTypeNameStruct ||
				col.TypeName == sql.ColumnInfoTypeNameMap ||
				col.TypeName == sql.ColumnInfoTypeNameArray
			columns = append(columns, columnMeta{
				name:      col.Name,
				typeName:  col.TypeName,
				isComplex: isComplex,
			})
		}
	}

	// Extract column names
	columnNames := make([]string, len(columns))
	for i, col := range columns {
		columnNames[i] = col.name
	}

	// "raw" format needs all rows buffered
	if format == "raw" {
		rows := collectAllRows(ctx, client, response, columns)
		output := map[string]interface{}{
			"statement_id": response.StatementId,
			"status":       response.Status.State,
			"columns":      columnNames,
			"rows":         rows,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return len(rows), enc.Encode(output)
	}

	// Stream rows chunk-by-chunk for json and csv formats
	rowCount := 0
	writeChunk := newChunkWriter(columnNames, format)
	if response.Result != nil {
		rows := convertRows(response.Result.DataArray, columns)
		if err := writeChunk(rows); err != nil {
			return 0, err
		}
		rowCount += len(rows)

		nextChunk := response.Result.NextChunkIndex
		for nextChunk > 0 {
			if CLI.Debug {
				fmt.Fprintf(os.Stderr, "DEBUG: fetching chunk %d\n", nextChunk)
			}
			chunk, err := client.StatementExecution.GetStatementResultChunkN(ctx, sql.GetStatementResultChunkNRequest{
				StatementId: response.StatementId,
				ChunkIndex:  nextChunk,
			})
			if err != nil {
				return 0, fmt.Errorf("failed to fetch chunk %d: %w", nextChunk, err)
			}
			rows := convertRows(chunk.DataArray, columns)
			if err := writeChunk(rows); err != nil {
				return 0, err
			}
			rowCount += len(rows)
			nextChunk = chunk.NextChunkIndex
		}
	}
	return rowCount, nil
}

// collectAllRows fetches all result rows including paginated chunks.
func collectAllRows(ctx context.Context, client *databricks.WorkspaceClient, response *sql.StatementResponse, columns []columnMeta) []map[string]interface{} {
	var rows []map[string]interface{}
	if response.Result != nil {
		rows = append(rows, convertRows(response.Result.DataArray, columns)...)
		nextChunk := response.Result.NextChunkIndex
		for nextChunk > 0 {
			if CLI.Debug {
				fmt.Fprintf(os.Stderr, "DEBUG: fetching chunk %d\n", nextChunk)
			}
			chunk, err := client.StatementExecution.GetStatementResultChunkN(ctx, sql.GetStatementResultChunkNRequest{
				StatementId: response.StatementId,
				ChunkIndex:  nextChunk,
			})
			if err != nil {
				break
			}
			rows = append(rows, convertRows(chunk.DataArray, columns)...)
			nextChunk = chunk.NextChunkIndex
		}
	}
	return rows
}

// newChunkWriter returns a function that writes a batch of rows in the selected format.
func newChunkWriter(columnNames []string, format string) func([]map[string]interface{}) error {
	headerWritten := false
	return func(rows []map[string]interface{}) error {
		result := &queryResult{columns: columnNames, rows: rows}
		switch format {
		case "csv":
			if !headerWritten {
				if err := result.writeCSV(os.Stdout); err != nil {
					return fmt.Errorf("CSV write error: %w", err)
				}
				headerWritten = true
			} else {
				if err := result.writeCSVRows(os.Stdout); err != nil {
					return fmt.Errorf("CSV write error: %w", err)
				}
			}
		default:
			if err := result.writeJSONL(os.Stdout); err != nil {
				return fmt.Errorf("JSONL write error: %w", err)
			}
		}
		return nil
	}
}
