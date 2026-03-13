package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/databricks/databricks-sdk-go"
	"github.com/databricks/databricks-sdk-go/service/sql"
)

// SQLCmd executes SQL queries
type SQLCmd struct {
	Query         string `arg:"" optional:"" help:"SQL query (or @file.sql)"`
	Format        string `short:"f" default:"json" help:"Output format (json, csv, raw)"`
	Limit         int64  `short:"l" default:"1000" help:"Maximum number of rows to return"`
	Timeout       int    `short:"t" default:"30" help:"Query timeout in seconds (5-50)"`
	Use           string `short:"u" help:"Default catalog[.schema] for query"`
	Async         bool   `help:"Submit query and return immediately, printing statement ID"`
}

func (c *SQLCmd) Run() error {
	host, err := getWorkspaceHost()
	if err != nil {
		return err
	}

	var query string
	if c.Query != "" {
		query = c.Query
		if strings.HasPrefix(query, "@") {
			data, err := os.ReadFile(query[1:])
			if err != nil {
				return fmt.Errorf("failed to read SQL file: %w", err)
			}
			query = string(data)
		}
	} else {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		query = string(data)
	}

	client, err := getAuthenticatedClient(host)
	if err != nil {
		return err
	}

	warehouseID, err := getWarehouseID(client)
	if err != nil {
		return err
	}

	if CLI.Debug {
		fmt.Fprintf(os.Stderr, "DEBUG: host=%s warehouse=%s\n", host, warehouseID)
		fmt.Fprintf(os.Stderr, "DEBUG: executing SQL:\n%s\n", query)
	}

	// Parse catalog[.schema] from --use flag
	var catalog, schema string
	if c.Use != "" {
		parts := strings.SplitN(c.Use, ".", 2)
		catalog = parts[0]
		if len(parts) > 1 {
			schema = parts[1]
		}
	}

	request := sql.ExecuteStatementRequest{
		WarehouseId: warehouseID,
		Statement:   query,
		RowLimit:    c.Limit,
		Catalog:     catalog,
		Schema:      schema,
	}
	if c.Async {
		request.WaitTimeout = "0s"
		request.OnWaitTimeout = sql.ExecuteStatementRequestOnWaitTimeoutContinue
	} else {
		request.WaitTimeout = fmt.Sprintf("%ds", c.Timeout)
	}

	ctx := context.Background()
	start := time.Now()
	response, err := client.StatementExecution.ExecuteStatement(ctx, request)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	state := response.Status.State
	switch state {
	case sql.StatementStateSucceeded:
		// Output results as normal
	case sql.StatementStatePending, sql.StatementStateRunning:
		fmt.Println(response.StatementId)
		fmt.Fprintf(os.Stderr, "Query is still %s. Check status with: dbq query status %s\n", strings.ToLower(string(state)), response.StatementId)
		return nil
	case sql.StatementStateFailed:
		if response.Status.Error != nil {
			return fmt.Errorf("query failed: %s", response.Status.Error.Message)
		}
		return fmt.Errorf("query failed")
	case sql.StatementStateCanceled:
		return fmt.Errorf("query was canceled (timed out after %ds; use --async to run asynchronously)", c.Timeout)
	default:
		return fmt.Errorf("unexpected query state: %s", state)
	}

	rowCount, err := outputResult(ctx, client, response, c.Format)
	if err != nil {
		return err
	}

	if CLI.Debug {
		elapsed := time.Since(start).Truncate(time.Millisecond)
		fmt.Fprintf(os.Stderr, "DEBUG: %d rows in %s\n", rowCount, elapsed)
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
