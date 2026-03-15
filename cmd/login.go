package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// AuthCmd groups authentication subcommands
type AuthCmd struct {
	Login  AuthLoginCmd  `cmd:"" help:"Authenticate with Databricks"`
	Status AuthStatusCmd `cmd:"" help:"Show current authentication status"`
}

// AuthLoginCmd authenticates with Databricks
type AuthLoginCmd struct{}

func (c *AuthLoginCmd) Run() error {
	host, err := getWorkspaceHost()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := doOAuthU2MLogin(ctx, host); err != nil {
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

// AuthStatusCmd shows the current authentication status
type AuthStatusCmd struct{}

func (c *AuthStatusCmd) Run() error {
	host, err := getWorkspaceHost()
	if err != nil {
		return err
	}

	client, err := newWorkspaceClient(host)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()
	user, err := client.CurrentUser.Me(ctx)
	if err != nil {
		return fmt.Errorf("not authenticated to %s (try \"dbq auth login\"): %w", host, err)
	}

	status := struct {
		Workspace string     `json:"workspace"`
		User      string     `json:"user"`
		Expiry    *time.Time `json:"expiry,omitempty"`
	}{
		Workspace: host,
		User:      user.UserName,
	}

	token, err := client.Config.GetTokenSource().Token(ctx)
	if err == nil && !token.Expiry.IsZero() {
		status.Expiry = &token.Expiry
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(status)
}
