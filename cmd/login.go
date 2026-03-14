package cmd

import (
	"context"
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

	fmt.Printf("Workspace: %s\n", host)
	fmt.Printf("User:      %s\n", user.UserName)

	token, err := client.Config.GetTokenSource().Token(ctx)
	if err == nil && !token.Expiry.IsZero() {
		remaining := time.Until(token.Expiry).Truncate(time.Second)
		if remaining > 0 {
			fmt.Printf("Expires:   in %s\n", remaining)
		} else {
			fmt.Printf("Expires:   expired %s ago\n", (-remaining).String())
		}
	}

	return nil
}
