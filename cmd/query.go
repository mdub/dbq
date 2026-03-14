package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/databricks/databricks-sdk-go/service/sql"
	"github.com/mdub/dbq/output"
	"github.com/mdub/dbq/result"
)

// QueryCmd groups query management subcommands
type QueryCmd struct {
	Status QueryStatusCmd `cmd:"" help:"Check status of an async query"`
	Fetch  QueryFetchCmd  `cmd:"" help:"Fetch results of a completed query"`
}

// QueryStatusCmd checks the status of a submitted statement
type QueryStatusCmd struct {
	StatementID string `arg:"" help:"Statement ID to check"`
}

func (c *QueryStatusCmd) Run() error {
	host, err := getWorkspaceHost()
	if err != nil {
		return err
	}

	client, err := getAuthenticatedClient(host)
	if err != nil {
		return err
	}

	ctx := context.Background()
	response, err := client.StatementExecution.GetStatement(ctx, sql.GetStatementRequest{
		StatementId: c.StatementID,
	})
	if err != nil {
		return fmt.Errorf("failed to get statement: %w", err)
	}

	state := response.Status.State
	fmt.Println(state)
	if state == sql.StatementStateFailed && response.Status.Error != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", response.Status.Error.Message)
	}
	if state == sql.StatementStateSucceeded {
		fmt.Fprintf(os.Stderr, "Fetch results with: dbq query fetch %s\n", c.StatementID)
	}
	return nil
}

// QueryFetchCmd fetches results of a completed statement
type QueryFetchCmd struct {
	StatementID string `arg:"" help:"Statement ID to fetch results for"`
	Format      string `short:"f" default:"json" help:"Output format (json, csv)"`
}

func (c *QueryFetchCmd) Run() error {
	host, err := getWorkspaceHost()
	if err != nil {
		return err
	}

	client, err := getAuthenticatedClient(host)
	if err != nil {
		return err
	}

	ctx := context.Background()
	response, err := client.StatementExecution.GetStatement(ctx, sql.GetStatementRequest{
		StatementId: c.StatementID,
	})
	if err != nil {
		return fmt.Errorf("failed to get statement: %w", err)
	}

	state := response.Status.State
	switch state {
	case sql.StatementStateSucceeded:
		// Good, fetch results below
	case sql.StatementStatePending, sql.StatementStateRunning:
		return fmt.Errorf("query is still %s; try again later", strings.ToLower(string(state)))
	case sql.StatementStateFailed:
		if response.Status.Error != nil {
			return fmt.Errorf("query failed: %s", response.Status.Error.Message)
		}
		return fmt.Errorf("query failed")
	default:
		return fmt.Errorf("query state is %s", strings.ToLower(string(state)))
	}

	qr := result.NewArrowResult(ctx, client, response, CLI.Debug)
	_, err = output.WriteResult(os.Stdout, qr, c.Format)
	return err
}
