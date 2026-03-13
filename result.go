package main

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
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

// QueryResult provides access to query result data.
type QueryResult interface {
	StatementID() string
	ColumnNames() []string
	Chunks() iter.Seq2[[]map[string]interface{}, error]
}

// newQueryResult creates a QueryResult from a Databricks API response.
func newQueryResult(ctx context.Context, client *databricks.WorkspaceClient, response *sql.StatementResponse) QueryResult {
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
	return &databricksResult{
		ctx:      ctx,
		client:   client,
		response: response,
		columns:  columns,
	}
}

// databricksResult implements QueryResult by fetching paginated chunks
// from the Databricks Statement Execution API.
type databricksResult struct {
	ctx      context.Context
	client   *databricks.WorkspaceClient
	response *sql.StatementResponse
	columns  []columnMeta
}

func (r *databricksResult) StatementID() string {
	return r.response.StatementId
}

func (r *databricksResult) ColumnNames() []string {
	names := make([]string, len(r.columns))
	for i, col := range r.columns {
		names[i] = col.name
	}
	return names
}

func (r *databricksResult) Chunks() iter.Seq2[[]map[string]interface{}, error] {
	return func(yield func([]map[string]interface{}, error) bool) {
		if r.response.Result == nil {
			return
		}
		rows := convertRows(r.response.Result.DataArray, r.columns)
		if !yield(rows, nil) {
			return
		}
		nextChunk := r.response.Result.NextChunkIndex
		for nextChunk > 0 {
			if CLI.Debug {
				fmt.Fprintf(os.Stderr, "DEBUG: fetching chunk %d\n", nextChunk)
			}
			chunk, err := r.client.StatementExecution.GetStatementResultChunkN(r.ctx, sql.GetStatementResultChunkNRequest{
				StatementId: r.response.StatementId,
				ChunkIndex:  nextChunk,
			})
			if err != nil {
				yield(nil, fmt.Errorf("failed to fetch chunk %d: %w", nextChunk, err))
				return
			}
			rows := convertRows(chunk.DataArray, r.columns)
			if !yield(rows, nil) {
				return
			}
			nextChunk = chunk.NextChunkIndex
		}
	}
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
