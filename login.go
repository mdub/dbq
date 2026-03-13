package main

import (
	"context"
	"fmt"
	"os"
)

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
