package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/databricks/databricks-sdk-go/service/sql"
	"github.com/mdub/dbq/output"
	"github.com/mdub/dbq/result"
)

// SQLCmd executes SQL queries
type SQLCmd struct {
	Query   string `arg:"" optional:"" help:"SQL query (or @file.sql)"`
	Format  string `short:"f" default:"json" help:"Output format (json, csv)"`
	Limit   int64  `short:"l" default:"1000" help:"Maximum number of rows to return"`
	Timeout int    `short:"t" default:"30" help:"Query timeout in seconds (5-50)"`
	Use     string `short:"u" help:"Default catalog[.schema] for query"`
	Async   bool   `help:"Submit query and return immediately, printing statement ID"`
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
		fmt.Fprintf(os.Stderr, "DEBUG: executing SQL:\n%s\n", indentSQL(query))
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
		Format:      sql.FormatArrowStream,
		Disposition: sql.DispositionExternalLinks,
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

	if CLI.Debug {
		fmt.Fprintf(os.Stderr, "DEBUG: statement_id=%s\n", response.StatementId)
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

	qr := result.NewArrowResult(ctx, client, response, CLI.Debug)
	rowCount, err := output.WriteResult(os.Stdout, qr, c.Format)
	if err != nil {
		return err
	}

	if CLI.Debug {
		elapsed := time.Since(start).Truncate(time.Millisecond)
		fmt.Fprintf(os.Stderr, "DEBUG: %d rows in %s\n", rowCount, elapsed)
	}
	return nil
}

// indentSQL formats SQL for debug output by stripping leading/trailing blank
// lines and indenting each line by two spaces.
func indentSQL(sql string) string {
	lines := strings.Split(sql, "\n")
	// strip leading blank lines
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	// strip trailing blank lines
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}
