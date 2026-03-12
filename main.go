package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/databricks/databricks-sdk-go"
	"github.com/databricks/databricks-sdk-go/credentials/u2m"
	"github.com/databricks/databricks-sdk-go/service/sql"
	"golang.org/x/term"
)

var CLI struct {
	Host        string `short:"H" env:"DATABRICKS_HOST" help:"Databricks host URL"`
	Warehouse   string `short:"w" env:"DBQ_WAREHOUSE" help:"SQL warehouse ID or name"`
	AutoLogin   bool `help:"Auto re-authenticate on auth failure"`
	NoAutoLogin bool `help:"Disable auto re-authentication"`
	Debug       bool   `help:"Enable debug output"`

	SQL        SQLCmd        `cmd:"" help:"Execute SQL query"`
	Warehouses WarehousesCmd `cmd:"" help:"List SQL warehouses"`
	Login      LoginCmd      `cmd:"" help:"Authenticate with Databricks"`
}

func getHost() (string, error) {
	if CLI.Host == "" {
		return "", fmt.Errorf("no host specified. Use --host or $DATABRICKS_HOST")
	}
	host := CLI.Host
	// Support simple names like "block-lakehouse-staging"
	if !strings.Contains(host, ".") {
		host = host + ".cloud.databricks.com"
	}
	if !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}
	return strings.TrimSuffix(host, "/"), nil
}

func newWorkspaceClient(host string) (*databricks.WorkspaceClient, error) {
	return databricks.NewWorkspaceClient(&databricks.Config{
		Host: host,
	})
}

func shouldAutoLogin() bool {
	if CLI.AutoLogin {
		return true
	}
	if CLI.NoAutoLogin {
		return false
	}
	return term.IsTerminal(int(os.Stderr.Fd()))
}

func doLogin(ctx context.Context, host string) error {
	arg, err := u2m.NewBasicWorkspaceOAuthArgument(host)
	if err != nil {
		return fmt.Errorf("invalid host: %w", err)
	}
	auth, err := u2m.NewPersistentAuth(ctx, u2m.WithOAuthArgument(arg))
	if err != nil {
		return fmt.Errorf("failed to create auth: %w", err)
	}
	return auth.Challenge()
}

func getAuthenticatedClient(host string) (*databricks.WorkspaceClient, error) {
	client, err := newWorkspaceClient(host)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	_, err = client.CurrentUser.Me(ctx)
	if err == nil {
		return client, nil
	}

	if !shouldAutoLogin() {
		return nil, fmt.Errorf("authentication failed (try \"dbq login --host %s\" to re-authenticate): %w", CLI.Host, err)
	}

	fmt.Fprintf(os.Stderr, "Authentication required. Logging in to %s ...\n", host)
	if loginErr := doLogin(ctx, host); loginErr != nil {
		return nil, fmt.Errorf("auto-login failed: %w", loginErr)
	}

	// Retry with fresh credentials
	client, err = newWorkspaceClient(host)
	if err != nil {
		return nil, err
	}

	return client, nil
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
	host, err := getHost()
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

func (r *queryResult) writeJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r.rows)
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

	// Collect all rows, fetching additional chunks if needed
	var rows []map[string]interface{}
	if response.Result != nil {
		rows = append(rows, convertRows(response.Result.DataArray, columns)...)

		// Fetch remaining chunks
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
			rows = append(rows, convertRows(chunk.DataArray, columns)...)
			nextChunk = chunk.NextChunkIndex
		}
	}

	// Extract column names
	columnNames := make([]string, len(columns))
	for i, col := range columns {
		columnNames[i] = col.name
	}

	result := &queryResult{columns: columnNames, rows: rows}

	switch c.Format {
	case "csv":
		if err := result.writeCSV(os.Stdout); err != nil {
			return fmt.Errorf("CSV write error: %w", err)
		}
	case "raw":
		output := map[string]interface{}{
			"statement_id": response.StatementId,
			"status":       response.Status.State,
			"columns":      columnNames,
			"rows":         rows,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	default:
		return result.writeJSON(os.Stdout)
	}
	return nil
}

// WarehousesCmd lists SQL warehouses
type WarehousesCmd struct{}

func (c *WarehousesCmd) Run() error {
	host, err := getHost()
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
	host, err := getHost()
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

func main() {
	ctx := kong.Parse(&CLI,
		kong.Name("dbq"),
		kong.Description("Databricks SQL query tool"),
		kong.UsageOnError(),
	)
	if CLI.AutoLogin && CLI.NoAutoLogin {
		ctx.Fatalf("cannot specify both --auto-login and --no-auto-login")
	}
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
