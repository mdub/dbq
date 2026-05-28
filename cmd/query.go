package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/databricks/databricks-sdk-go"
	"github.com/databricks/databricks-sdk-go/service/iam"
	"github.com/databricks/databricks-sdk-go/service/sql"
	"github.com/mdub/dbq/metrics"
	"github.com/mdub/dbq/result"
)

// QueryCmd groups query management subcommands
type QueryCmd struct {
	Status  QueryStatusCmd  `cmd:"" help:"Check status of an async query"`
	Wait    QueryWaitCmd    `cmd:"" help:"Wait for a query to complete"`
	Cancel  QueryCancelCmd  `cmd:"" help:"Cancel a running query"`
	Results QueryResultsCmd `cmd:"" help:"Fetch results of a completed query"`
	Metrics QueryMetricsCmd `cmd:"" help:"Show execution metrics for a query"`
	Info    QueryInfoCmd    `cmd:"" help:"Show query details (SQL text, status, timings) from query history"`
	List    QueryListCmd    `cmd:"" help:"List recent queries from query history"`
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
		fmt.Fprintf(os.Stderr, "Fetch results with: dbq query results %s\n", c.StatementID)
	}
	return nil
}

// QueryWaitCmd polls until a query reaches a terminal state.
type QueryWaitCmd struct {
	StatementID     string  `arg:"" help:"Statement ID to wait for"`
	Timeout         int     `short:"t" default:"300" help:"Maximum seconds to wait"`
	PollInterval    float64 `short:"i" default:"5" help:"Poll interval in seconds"`
	CancelOnTimeout bool    `help:"Cancel the query if the timeout is reached"`
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
	pollInterval := time.Duration(c.PollInterval * float64(time.Second))

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
		if time.Now().Add(pollInterval).After(deadline) {
			if c.CancelOnTimeout {
				cancelErr := client.StatementExecution.CancelExecution(ctx, sql.CancelExecutionRequest{
					StatementId: c.StatementID,
				})
				if cancelErr != nil {
					return fmt.Errorf("timed out after %ds; cancel failed: %w", c.Timeout, cancelErr)
				}
				return fmt.Errorf("timed out after %ds; query canceled", c.Timeout)
			}
			return fmt.Errorf("timed out after %ds waiting for query to complete (state: %s)", c.Timeout, strings.ToLower(string(state)))
		}
		logDebug("waiting... (%s)", strings.ToLower(string(state)))
		time.Sleep(pollInterval)
	}
}

// QueryCancelCmd cancels a running query.
type QueryCancelCmd struct {
	StatementID string `arg:"" help:"Statement ID to cancel"`
}

func (c *QueryCancelCmd) Run() error {
	host, err := getWorkspaceHost()
	if err != nil {
		return err
	}

	client, err := getAuthenticatedClient(host)
	if err != nil {
		return err
	}

	ctx := context.Background()
	err = client.StatementExecution.CancelExecution(ctx, sql.CancelExecutionRequest{
		StatementId: c.StatementID,
	})
	if err != nil {
		return fmt.Errorf("failed to cancel query: %w", err)
	}
	return nil
}

// QueryResultsCmd fetches results of a completed statement
type QueryResultsCmd struct {
	StatementID   string `arg:"" help:"Statement ID to fetch results for"`
	OutputOptions `embed:""`
}

func (c *QueryResultsCmd) Run() error {
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

	qr := result.NewArrowResult(ctx, client, response, logDebug)
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

// QueryInfoCmd shows query details (including the SQL text) from query history.
type QueryInfoCmd struct {
	StatementID string `arg:"" help:"Statement ID to look up"`
}

func (c *QueryInfoCmd) Run() error {
	host, err := getWorkspaceHost()
	if err != nil {
		return err
	}
	client, err := getAuthenticatedClient(host)
	if err != nil {
		return err
	}

	ctx := context.Background()
	response, err := client.QueryHistory.List(ctx, sql.ListQueryHistoryRequest{
		FilterBy: &sql.QueryFilter{
			StatementIds: []string{c.StatementID},
		},
		IncludeMetrics: true,
		MaxResults:     1,
	})
	if err != nil {
		return fmt.Errorf("failed to look up query: %w", err)
	}
	if len(response.Res) == 0 {
		return fmt.Errorf("no query found with statement ID %s", c.StatementID)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(response.Res[0])
}

// QueryListCmd lists recent queries from query history.
type QueryListCmd struct {
	MaxResults  int      `short:"n" default:"20" help:"Maximum number of queries to return (max 1000)"`
	Status      []string `help:"Filter by status (QUEUED, RUNNING, CANCELED, FAILED, FINISHED)"`
	WarehouseID []string `help:"Filter by warehouse ID"`
	User        string   `help:"Filter by numeric user ID; 'all' (or '*') for no filter; default is the current user" placeholder:"ID"`
	Summary     bool     `short:"s" help:"Print one summary line per query instead of full JSON"`
}

func (c *QueryListCmd) Run() error {
	host, err := getWorkspaceHost()
	if err != nil {
		return err
	}
	client, err := getAuthenticatedClient(host)
	if err != nil {
		return err
	}

	ctx := context.Background()

	filter := &sql.QueryFilter{
		WarehouseIds: c.WarehouseID,
	}
	for _, s := range c.Status {
		filter.Statuses = append(filter.Statuses, sql.QueryStatus(strings.ToUpper(s)))
	}
	if c.User != "all" && c.User != "*" {
		uid, err := resolveUserID(ctx, client, c.User)
		if err != nil {
			return err
		}
		filter.UserIds = []int64{uid}
	}
	response, err := client.QueryHistory.List(ctx, sql.ListQueryHistoryRequest{
		FilterBy:   filter,
		MaxResults: c.MaxResults,
	})
	if err != nil {
		return fmt.Errorf("failed to list queries: %w", err)
	}

	if c.Summary {
		const format = "%-36s  %-36s  %-20s  %10s  %-9s\n"
		fmt.Printf(format, "USER", "QUERY ID", "STARTED", "DURATION", "STATUS")
		for _, q := range response.Res {
			started := ""
			if q.QueryStartTimeMs > 0 {
				started = time.UnixMilli(q.QueryStartTimeMs).UTC().Format("2006-01-02T15:04:05Z")
			}
			duration := (time.Duration(q.Duration) * time.Millisecond).Round(time.Millisecond)
			fmt.Printf(format,
				q.UserName,
				q.QueryId,
				started,
				duration,
				q.Status,
			)
		}
		return nil
	}

	enc := json.NewEncoder(os.Stdout)
	for _, q := range response.Res {
		if err := enc.Encode(q); err != nil {
			return err
		}
	}
	return nil
}


// resolveUserID maps c.User (empty | numeric ID | userName | service principal applicationId)
// to a numeric Databricks user ID suitable for QueryFilter.UserIds.
func resolveUserID(ctx context.Context, client *databricks.WorkspaceClient, input string) (int64, error) {
	if input == "" {
		me, err := client.CurrentUser.Me(ctx, iam.MeRequest{})
		if err != nil {
			return 0, fmt.Errorf("failed to look up current user: %w", err)
		}
		return parseID(me.Id, "current user")
	}
	if id, err := strconv.ParseInt(input, 10, 64); err == nil {
		return id, nil
	}

	users, err := client.Users.ListAll(ctx, iam.ListUsersRequest{
		Filter:     fmt.Sprintf("userName eq %q", input),
		Attributes: "id,userName",
	})
	if err != nil {
		return 0, fmt.Errorf("failed to look up user %q: %w", input, err)
	}
	if len(users) > 0 {
		return parseID(users[0].Id, "user "+input)
	}

	sps, err := client.ServicePrincipals.ListAll(ctx, iam.ListServicePrincipalsRequest{
		Filter:     fmt.Sprintf("applicationId eq %q", input),
		Attributes: "id,applicationId",
	})
	if err != nil {
		return 0, fmt.Errorf("failed to look up service principal %q: %w", input, err)
	}
	if len(sps) > 0 {
		return parseID(sps[0].Id, "service principal "+input)
	}

	return 0, fmt.Errorf("no user or service principal found matching %q", input)
}

func parseID(s, what string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("could not parse %s ID %q: %w", what, s, err)
	}
	return id, nil
}
