package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/databricks/databricks-sdk-go/service/sql"
	"github.com/mdub/dbq/result"
)

// SQLCmd executes SQL queries
type SQLCmd struct {
	Query         string `arg:"" optional:"" help:"SQL query (or @file.sql)"`
	OutputOptions `embed:""`
	Limit         int64    `short:"l" default:"1000" help:"Maximum number of rows to return"`
	Timeout       *int     `short:"t" help:"Query timeout in seconds (5-50, default 30); ignored with --async"`
	Use           string   `short:"u" help:"Default catalog[.schema] for query"`
	Async         bool     `help:"Submit query and return immediately, printing statement ID"`
	Tag           []string `help:"Query tag (KEY=VALUE), repeatable" placeholder:"KEY=VALUE"`
}

func (c *SQLCmd) Run() error {
	if c.Async && c.Timeout != nil {
		return fmt.Errorf("--timeout has no effect with --async; use `dbq query wait -t N` to bound the wait")
	}
	timeout := 30
	if c.Timeout != nil {
		timeout = *c.Timeout
	}

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

	logDebug("host=%s warehouse=%s", host, warehouseID)
	logDebug("executing SQL:\n%s", indentSQL(query))

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
	for _, t := range c.Tag {
		k, v, ok := strings.Cut(t, "=")
		if !ok || k == "" {
			return fmt.Errorf("invalid --tag %q: expected KEY=VALUE", t)
		}
		request.QueryTags = append(request.QueryTags, sql.QueryTag{Key: k, Value: v})
	}
	if c.Async {
		request.WaitTimeout = "0s"
		request.OnWaitTimeout = sql.ExecuteStatementRequestOnWaitTimeoutContinue
	} else {
		request.WaitTimeout = fmt.Sprintf("%ds", timeout)
		request.OnWaitTimeout = sql.ExecuteStatementRequestOnWaitTimeoutCancel
	}

	ctx := context.Background()
	response, err := client.StatementExecution.ExecuteStatement(ctx, request)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	logDebug("statement_id=%s", response.StatementId)

	state := response.Status.State
	switch state {
	case sql.StatementStateSucceeded:
		// Output results below
	case sql.StatementStatePending, sql.StatementStateRunning:
		logDebug("query is still %s", strings.ToLower(string(state)))
		fmt.Println(response.StatementId)
		return nil
	case sql.StatementStateFailed:
		if response.Status.Error != nil {
			return fmt.Errorf("query %s failed: %s", response.StatementId, response.Status.Error.Message)
		}
		return fmt.Errorf("query %s failed", response.StatementId)
	case sql.StatementStateCanceled:
		return fmt.Errorf("query %s timed out after %ds (raise --timeout, or use --async + `dbq query wait`)", response.StatementId, timeout)
	default:
		return fmt.Errorf("query %s has unexpected state: %s", response.StatementId, state)
	}

	qr := result.NewArrowResult(ctx, client, response, logDebug)
	rowCount, err := c.OutputOptions.WriteResult(qr)
	if err != nil {
		return err
	}

	logDebug("%d rows", rowCount)
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
