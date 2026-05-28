package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/databricks/databricks-sdk-go"
	"github.com/databricks/databricks-sdk-go/config"
	"github.com/databricks/databricks-sdk-go/config/credentials"
	"github.com/databricks/databricks-sdk-go/config/experimental/auth"
	"github.com/databricks/databricks-sdk-go/credentials/u2m"
	"github.com/databricks/databricks-sdk-go/service/iam"
	"golang.org/x/oauth2"
	"golang.org/x/term"
)

func newWorkspaceClient(host string) (*databricks.WorkspaceClient, error) {
	return databricks.NewWorkspaceClient(&databricks.Config{
		Host:        host,
		Credentials: dbqCredentials{host: host},
	})
}

// dbqCredentials is a CredentialsStrategy that prefers env-var-based
// authentication (PAT, OAuth M2M) and falls back to U2M via our own
// PersistentAuth token cache — bypassing the SDK's `databricks-cli` strategy,
// which shells out to the `databricks` CLI with `--force-refresh` and can
// open a browser on every call.
type dbqCredentials struct {
	host string
}

func (dbqCredentials) Name() string { return "dbq" }

func (c dbqCredentials) Configure(ctx context.Context, cfg *config.Config) (credentials.CredentialsProvider, error) {
	if err := cfg.EnsureResolved(); err != nil {
		return nil, err
	}
	if cfg.Token != "" {
		return config.PatCredentials{}.Configure(ctx, cfg)
	}
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		return config.M2mCredentials{}.Configure(ctx, cfg)
	}

	pa, err := newPersistentAuth(ctx, c.host)
	if err != nil {
		return nil, fmt.Errorf("failed to create persistent auth: %w", err)
	}
	ts := auth.TokenSourceFn(func(context.Context) (*oauth2.Token, error) {
		return pa.Token()
	})
	return credentials.NewOAuthCredentialsProviderFromTokenSource(ts), nil
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

func newPersistentAuth(ctx context.Context, host string) (*u2m.PersistentAuth, error) {
	arg, err := u2m.NewBasicWorkspaceOAuthArgument(host)
	if err != nil {
		return nil, fmt.Errorf("invalid host: %w", err)
	}
	tc, err := newDefaultTokenCache()
	if err != nil {
		return nil, err
	}
	return u2m.NewPersistentAuth(ctx,
		u2m.WithOAuthArgument(arg),
		u2m.WithTokenCache(tc),
	)
}

func doOAuthU2MLogin(ctx context.Context, host string) error {
	logDebug("attempting OAuth U2M login to %s ...", host)
	pa, err := newPersistentAuth(ctx, host)
	if err != nil {
		return err
	}
	return pa.Challenge()
}

func getAuthenticatedClient(host string) (*databricks.WorkspaceClient, error) {
	client, err := newWorkspaceClient(host)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	_, err = client.CurrentUser.Me(ctx, iam.MeRequest{})
	if err == nil {
		return client, nil
	}

	if !shouldAutoLogin() {
		return nil, fmt.Errorf("authentication failed (try \"dbq auth login\", or set DATABRICKS_TOKEN): %w", err)
	}

	if loginErr := doOAuthU2MLogin(ctx, host); loginErr != nil {
		return nil, fmt.Errorf("auto-login failed: %w", loginErr)
	}

	return client, nil
}
