package main

import (
	"context"
	"fmt"
	"os"

	"github.com/databricks/databricks-sdk-go"
	"github.com/databricks/databricks-sdk-go/credentials/u2m"
	"golang.org/x/term"
)

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
		return nil, fmt.Errorf("authentication failed (try \"dbq login --workspace %s\" to re-authenticate): %w", CLI.Workspace, err)
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
