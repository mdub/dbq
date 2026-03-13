package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/databricks/databricks-sdk-go"
	"github.com/databricks/databricks-sdk-go/service/sql"
)

func getWorkspaceHost() (string, error) {
	if CLI.Workspace == "" {
		return "", fmt.Errorf("no workspace specified. Use --workspace or $DBQ_WORKSPACE")
	}
	host := CLI.Workspace
	// Support simple names like "my-workspace"
	if !strings.Contains(host, ".") {
		host = host + ".cloud.databricks.com"
	}
	if !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}
	return strings.TrimSuffix(host, "/"), nil
}

const defaultWarehouse = "Serverless Starter Warehouse"

func getWarehouseID(client *databricks.WorkspaceClient) (string, error) {
	warehouse := CLI.Warehouse
	if warehouse == "" {
		warehouse = defaultWarehouse
	}

	// If it looks like a warehouse ID (hex string), return as-is
	if isWarehouseID(warehouse) {
		return warehouse, nil
	}

	// Otherwise, look up by name
	ctx := context.Background()
	warehouses, err := client.Warehouses.ListAll(ctx, sql.ListWarehousesRequest{})
	if err != nil {
		return "", fmt.Errorf("failed to list warehouses: %w", err)
	}

	for _, wh := range warehouses {
		if wh.Name == warehouse {
			return wh.Id, nil
		}
	}

	return "", fmt.Errorf("warehouse not found: %s", warehouse)
}

func isWarehouseID(s string) bool {
	if len(s) < 16 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// SQLCmd executes SQL queries
type SQLCmd struct {
	Query   string `arg:"" optional:"" help:"SQL query (or @file.sql)"`
	Format  string `short:"f" default:"json" help:"Output format (json, csv, raw)"`
	Timeout int    `short:"t" default:"30" help:"Query timeout in seconds (5-50)"`
	Use     string `short:"u" help:"Default catalog[.schema] for query"`
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

	ctx := context.Background()
	response, err := client.StatementExecution.ExecuteAndWait(ctx, sql.ExecuteStatementRequest{
		WarehouseId: warehouseID,
		Statement:   query,
		WaitTimeout: fmt.Sprintf("%ds", c.Timeout),
		Catalog:     catalog,
		Schema:      schema,
	})
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return c.outputResult(ctx, client, response)
}

func (c *SQLCmd) outputResult(ctx context.Context, client *databricks.WorkspaceClient, response *sql.StatementResponse) error {
	if response.Status.State == sql.StatementStateFailed {
		return fmt.Errorf("query failed: %s", response.Status.Error.Message)
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
	if c.Format == "raw" {
		rows := c.collectAllRows(ctx, client, response, columns)
		output := map[string]interface{}{
			"statement_id": response.StatementId,
			"status":       response.Status.State,
			"columns":      columnNames,
			"rows":         rows,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	// Stream rows chunk-by-chunk for json and csv formats
	writeChunk := c.chunkWriter(columnNames)
	if response.Result != nil {
		rows := convertRows(response.Result.DataArray, columns)
		if err := writeChunk(rows); err != nil {
			return err
		}

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
				return fmt.Errorf("failed to fetch chunk %d: %w", nextChunk, err)
			}
			rows := convertRows(chunk.DataArray, columns)
			if err := writeChunk(rows); err != nil {
				return err
			}
			nextChunk = chunk.NextChunkIndex
		}
	}
	return nil
}

func (c *SQLCmd) collectAllRows(ctx context.Context, client *databricks.WorkspaceClient, response *sql.StatementResponse, columns []columnMeta) []map[string]interface{} {
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

// chunkWriter returns a function that writes a batch of rows in the selected format.
func (c *SQLCmd) chunkWriter(columnNames []string) func([]map[string]interface{}) error {
	headerWritten := false
	return func(rows []map[string]interface{}) error {
		result := &queryResult{columns: columnNames, rows: rows}
		switch c.Format {
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

// WarehousesCmd lists SQL warehouses
type WarehousesCmd struct{}

func (c *WarehousesCmd) Run() error {
	host, err := getWorkspaceHost()
	if err != nil {
		return err
	}

	client, err := getAuthenticatedClient(host)
	if err != nil {
		return err
	}

	ctx := context.Background()
	warehouses, err := client.Warehouses.ListAll(ctx, sql.ListWarehousesRequest{})
	if err != nil {
		return fmt.Errorf("failed to list warehouses: %w", err)
	}

	for _, wh := range warehouses {
		fmt.Printf("%-20s %-40s %s\n", wh.Id, wh.Name, wh.State)
	}
	return nil
}

// LoginCmd authenticates with Databricks
type LoginCmd struct{}

func (c *LoginCmd) Run() error {
	host, err := getWorkspaceHost()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Authenticating to %s ...\n", host)

	ctx := context.Background()
	if err := doLogin(ctx, host); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Verify by fetching current user
	client, err := newWorkspaceClient(host)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	user, err := client.CurrentUser.Me(ctx)
	if err != nil {
		return fmt.Errorf("failed to verify authentication: %w", err)
	}

	fmt.Printf("Authenticated as %s\n", user.UserName)
	return nil
}
