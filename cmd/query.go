package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/databricks/databricks-sdk-go/service/sql"
	"github.com/mdub/dbq/metrics"
	"github.com/mdub/dbq/result"
)

// QueryCmd groups query management subcommands
type QueryCmd struct {
	Status  QueryStatusCmd  `cmd:"" help:"Check status of an async query"`
	Wait    QueryWaitCmd    `cmd:"" help:"Wait for a query to complete"`
	Fetch   QueryFetchCmd   `cmd:"" help:"Fetch results of a completed query"`
	Metrics QueryMetricsCmd `cmd:"" help:"Show execution metrics for a query"`
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

// QueryWaitCmd polls until a query reaches a terminal state.
type QueryWaitCmd struct {
	StatementID string `arg:"" help:"Statement ID to wait for"`
	Timeout     int    `short:"t" default:"300" help:"Maximum seconds to wait"`
}

func (c *QueryWaitCmd) Run() error {
	host, err := getWorkspaceHost()
	if err != nil {
		return err
	}

	client, err := getAuthenticatedClient(host)
	if err != nil {
		return err
	}

	ctx := context.Background()
	deadline := time.Now().Add(time.Duration(c.Timeout) * time.Second)
	interval := 1 * time.Second
	maxInterval := 5 * time.Second

	for {
		response, err := client.StatementExecution.GetStatement(ctx, sql.GetStatementRequest{
			StatementId: c.StatementID,
		})
		if err != nil {
			return fmt.Errorf("failed to get statement: %w", err)
		}

		state := response.Status.State
		switch state {
		case sql.StatementStateSucceeded:
			return nil
		case sql.StatementStateFailed:
			if response.Status.Error != nil {
				return fmt.Errorf("query failed: %s", response.Status.Error.Message)
			}
			return fmt.Errorf("query failed")
		case sql.StatementStateCanceled:
			return fmt.Errorf("query was canceled")
		}

		// Still PENDING or RUNNING
		if time.Now().Add(interval).After(deadline) {
			return fmt.Errorf("timed out after %ds waiting for query to complete (state: %s)", c.Timeout, strings.ToLower(string(state)))
		}
		fmt.Fprintf(os.Stderr, "Waiting... (%s)\n", strings.ToLower(string(state)))
		time.Sleep(interval)
		if interval < maxInterval {
			interval *= 2
			if interval > maxInterval {
				interval = maxInterval
			}
		}
	}
}

// QueryFetchCmd fetches results of a completed statement
type QueryFetchCmd struct {
	StatementID   string `arg:"" help:"Statement ID to fetch results for"`
	OutputOptions `embed:""`
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

	qr := result.NewArrowResult(ctx, client, response, debugf())
	_, err = c.OutputOptions.WriteResult(qr)
	return err
}

// QueryMetricsCmd prints query execution metrics as JSON.
type QueryMetricsCmd struct {
	StatementID string `arg:"" help:"Statement ID to fetch metrics for"`
}

func (c *QueryMetricsCmd) Run() error {
	host, err := getWorkspaceHost()
	if err != nil {
		return err
	}
	client, err := getAuthenticatedClient(host)
	if err != nil {
		return err
	}
	ctx := context.Background()
	qm, err := metrics.Fetch(ctx, client, c.StatementID)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(qm)
}
