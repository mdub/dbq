package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/databricks/databricks-sdk-go"
	"github.com/databricks/databricks-sdk-go/credentials/u2m"
	"github.com/databricks/databricks-sdk-go/service/sql"
)

var CLI struct {
	Host      string `short:"H" env:"DATABRICKS_HOST" help:"Databricks host URL"`
	Warehouse string `short:"w" env:"DBQ_WAREHOUSE" help:"SQL warehouse ID or name"`
	Debug     bool   `help:"Enable debug output"`

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
	Query  string `arg:"" optional:"" help:"SQL query (or @file.sql)"`
	Format string `short:"f" default:"json" help:"Output format (json, csv, raw)"`
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

	client, err := newWorkspaceClient(host)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	warehouseID, err := getWarehouseID(client)
	if err != nil {
		return err
	}

	if CLI.Debug {
		fmt.Fprintf(os.Stderr, "DEBUG: host=%s warehouse=%s\n", host, warehouseID)
		fmt.Fprintf(os.Stderr, "DEBUG: executing SQL:\n%s\n", query)
	}

	ctx := context.Background()
	response, err := client.StatementExecution.ExecuteAndWait(ctx, sql.ExecuteStatementRequest{
		WarehouseId: warehouseID,
		Statement:   query,
		WaitTimeout: "30s",
	})
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return c.outputResult(response)
}

// columnMeta holds column name and type information
type columnMeta struct {
	name      string
	typeName  sql.ColumnInfoTypeName
	isComplex bool // true for STRUCT, MAP, ARRAY
}

func (c *SQLCmd) outputResult(response *sql.StatementResponse) error {
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

	var rows []map[string]interface{}
	if response.Result != nil && response.Result.DataArray != nil {
		for _, row := range response.Result.DataArray {
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
	}

	// Extract column names for output formats that need them
	columnNames := make([]string, len(columns))
	for i, col := range columns {
		columnNames[i] = col.name
	}

	switch c.Format {
	case "csv":
		if len(columnNames) > 0 {
			fmt.Println(strings.Join(columnNames, ","))
		}
		for _, row := range rows {
			var vals []string
			for _, name := range columnNames {
				vals = append(vals, fmt.Sprintf("%v", row[name]))
			}
			fmt.Println(strings.Join(vals, ","))
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
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
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

	client, err := newWorkspaceClient(host)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
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

	arg, err := u2m.NewBasicWorkspaceOAuthArgument(host)
	if err != nil {
		return fmt.Errorf("invalid host: %w", err)
	}

	auth, err := u2m.NewPersistentAuth(ctx, u2m.WithOAuthArgument(arg))
	if err != nil {
		return fmt.Errorf("failed to create auth: %w", err)
	}

	if err := auth.Challenge(); err != nil {
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
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
