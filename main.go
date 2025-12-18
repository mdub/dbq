package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/alecthomas/kong"
	"github.com/databricks/databricks-sdk-go"
	"github.com/databricks/databricks-sdk-go/credentials/u2m"
	"github.com/databricks/databricks-sdk-go/service/sql"
)

var CLI struct {
	Profile   string `short:"p" env:"DBQ_PROFILE" help:"Databricks profile"`
	Warehouse string `short:"w" help:"SQL warehouse ID"`
	Debug     bool   `help:"Enable debug output"`

	SQL        SQLCmd        `cmd:"" help:"Execute SQL query"`
	Warehouses WarehousesCmd `cmd:"" help:"List SQL warehouses"`
	Login      LoginCmd      `cmd:"" help:"Authenticate with Databricks"`
	Profile_   ProfileCmd    `cmd:"" name:"profile" help:"Manage profiles"`
}

// Profile represents a single profile in profiles.toml
type Profile struct {
	Host      string `toml:"host"`
	Warehouse string `toml:"warehouse,omitempty"`
}

// Config represents the profiles.toml file
type Config struct {
	Default  string             `toml:"default,omitempty"`
	Profiles map[string]Profile `toml:"-"`
}

func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "dbq")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dbq")
}

func configPath() string {
	return filepath.Join(configDir(), "profiles.toml")
}

func loadConfig() (*Config, error) {
	path := configPath()
	cfg := &Config{
		Profiles: make(map[string]Profile),
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil // Return empty config if file doesn't exist
	}

	// Decode all sections as raw data
	var raw map[string]interface{}
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Extract default if it's a string (not a section)
	if defaultVal, ok := raw["default"]; ok {
		if defaultStr, ok := defaultVal.(string); ok {
			cfg.Default = defaultStr
		}
		// If it's a map, it's a profile section named "default", handled below
	}

	// Parse all sections as profiles
	for name, val := range raw {
		if section, ok := val.(map[string]interface{}); ok {
			profile := Profile{}
			if host, ok := section["host"].(string); ok {
				profile.Host = host
			}
			if wh, ok := section["warehouse"].(string); ok {
				profile.Warehouse = wh
			}
			cfg.Profiles[name] = profile
		}
	}

	return cfg, nil
}

func saveConfig(cfg *Config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write default first if set
	if cfg.Default != "" {
		fmt.Fprintf(f, "default = %q\n\n", cfg.Default)
	}

	// Write each profile
	for name, profile := range cfg.Profiles {
		fmt.Fprintf(f, "[%s]\n", name)
		fmt.Fprintf(f, "host = %q\n", profile.Host)
		if profile.Warehouse != "" {
			fmt.Fprintf(f, "warehouse = %q\n", profile.Warehouse)
		}
		fmt.Fprintln(f)
	}

	return nil
}

func getProfileName(cfg *Config) (string, error) {
	// Priority: --profile flag > $DBQ_PROFILE > config default
	name := CLI.Profile
	if name == "" {
		name = cfg.Default
	}
	if name == "" {
		return "", fmt.Errorf("no profile specified. Use --profile, $DBQ_PROFILE, or set a default with: dbq profile default NAME")
	}
	return name, nil
}

func getProfile(cfg *Config, name string) (*Profile, error) {
	profile, ok := cfg.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("unknown profile: %s\n\nCreate it with: dbq profile add %s", name, name)
	}
	return &profile, nil
}

func newWorkspaceClient(profile *Profile) (*databricks.WorkspaceClient, error) {
	return databricks.NewWorkspaceClient(&databricks.Config{
		Host: profile.Host,
	})
}

func getWarehouseID(profile *Profile) string {
	if CLI.Warehouse != "" {
		return CLI.Warehouse
	}
	return profile.Warehouse
}

// ProfileCmd manages profiles
type ProfileCmd struct {
	List    ProfileListCmd    `cmd:"" help:"List available profiles"`
	Show    ProfileShowCmd    `cmd:"" help:"Show profile configuration"`
	Add     ProfileAddCmd     `cmd:"" help:"Add a new profile"`
	Default ProfileDefaultCmd `cmd:"" help:"Get or set the default profile"`
}

type ProfileListCmd struct{}

func (c *ProfileListCmd) Run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 {
		fmt.Println("No profiles configured.")
		fmt.Printf("\nCreate one with: dbq profile add NAME\n")
		return nil
	}
	for name := range cfg.Profiles {
		marker := " "
		if name == cfg.Default {
			marker = "*"
		}
		fmt.Printf("%s %s\n", marker, name)
	}
	return nil
}

type ProfileShowCmd struct {
	Name string `arg:"" help:"Profile name"`
}

func (c *ProfileShowCmd) Run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	profile, err := getProfile(cfg, c.Name)
	if err != nil {
		return err
	}
	fmt.Printf("host = %q\n", profile.Host)
	if profile.Warehouse != "" {
		fmt.Printf("warehouse = %q\n", profile.Warehouse)
	}
	if c.Name == cfg.Default {
		fmt.Printf("\n(default profile)\n")
	}
	return nil
}

type ProfileAddCmd struct {
	Name      string `arg:"" help:"Profile name"`
	Workspace string `arg:"" optional:"" help:"Workspace name (defaults to profile name)"`
}

func (c *ProfileAddCmd) Run() error {
	if c.Name == "default" {
		return fmt.Errorf("cannot create a profile named \"default\" (reserved)")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if _, exists := cfg.Profiles[c.Name]; exists {
		return fmt.Errorf("profile %q already exists", c.Name)
	}

	workspace := c.Workspace
	if workspace == "" {
		workspace = c.Name
	}

	host := fmt.Sprintf("https://%s.cloud.databricks.com", workspace)

	cfg.Profiles[c.Name] = Profile{
		Host: host,
	}

	// If this is the first profile, make it the default
	if len(cfg.Profiles) == 1 {
		cfg.Default = c.Name
	}

	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Added profile %q with host %s\n", c.Name, host)
	if cfg.Default == c.Name {
		fmt.Println("(set as default)")
	}
	return nil
}

type ProfileDefaultCmd struct {
	Name string `arg:"" optional:"" help:"Profile name to set as default"`
}

func (c *ProfileDefaultCmd) Run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// If no name given, show current default
	if c.Name == "" {
		if cfg.Default == "" {
			fmt.Println("No default profile set.")
			fmt.Printf("\nSet one with: dbq profile default NAME\n")
		} else {
			fmt.Println(cfg.Default)
		}
		return nil
	}

	// Verify profile exists
	if _, err := getProfile(cfg, c.Name); err != nil {
		return err
	}

	cfg.Default = c.Name
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Default profile set to %q\n", c.Name)
	return nil
}

// SQLCmd executes SQL queries
type SQLCmd struct {
	Query  string `arg:"" optional:"" help:"SQL query (or @file.sql)"`
	Format string `short:"f" default:"json" help:"Output format (json, csv, raw)"`
}

func (c *SQLCmd) Run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	profileName, err := getProfileName(cfg)
	if err != nil {
		return err
	}
	profile, err := getProfile(cfg, profileName)
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

	warehouseID := getWarehouseID(profile)
	if warehouseID == "" {
		return fmt.Errorf("no warehouse specified. Use --warehouse or set 'warehouse' in profile")
	}

	if CLI.Debug {
		fmt.Fprintf(os.Stderr, "DEBUG: profile=%s host=%s warehouse=%s\n", profileName, profile.Host, warehouseID)
		fmt.Fprintf(os.Stderr, "DEBUG: executing SQL:\n%s\n", query)
	}

	client, err := newWorkspaceClient(profile)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()
	response, err := client.StatementExecution.ExecuteAndWait(ctx, sql.ExecuteStatementRequest{
		WarehouseId: warehouseID,
		Statement:   query,
		WaitTimeout: "30s",
	})
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return c.outputResult(response)
}

func (c *SQLCmd) outputResult(response *sql.StatementResponse) error {
	if response.Status.State == sql.StatementStateFailed {
		return fmt.Errorf("query failed: %s", response.Status.Error.Message)
	}

	var columns []string
	if response.Manifest != nil && response.Manifest.Schema != nil {
		for _, col := range response.Manifest.Schema.Columns {
			columns = append(columns, col.Name)
		}
	}

	var rows []map[string]interface{}
	if response.Result != nil && response.Result.DataArray != nil {
		for _, row := range response.Result.DataArray {
			rowMap := make(map[string]interface{})
			for i, val := range row {
				if i < len(columns) {
					rowMap[columns[i]] = val
				}
			}
			rows = append(rows, rowMap)
		}
	}

	fmt.Printf("-- statement_id: %s, status: %s\n\n", response.StatementId, response.Status.State)

	switch c.Format {
	case "csv":
		if len(columns) > 0 {
			fmt.Println(strings.Join(columns, ","))
		}
		for _, row := range rows {
			var vals []string
			for _, col := range columns {
				vals = append(vals, fmt.Sprintf("%v", row[col]))
			}
			fmt.Println(strings.Join(vals, ","))
		}
	case "raw":
		output := map[string]interface{}{
			"statement_id": response.StatementId,
			"status":       response.Status.State,
			"columns":      columns,
			"rows":         rows,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	default:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	return nil
}

// WarehousesCmd lists SQL warehouses
type WarehousesCmd struct{}

func (c *WarehousesCmd) Run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	profileName, err := getProfileName(cfg)
	if err != nil {
		return err
	}
	profile, err := getProfile(cfg, profileName)
	if err != nil {
		return err
	}

	client, err := newWorkspaceClient(profile)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	ctx := context.Background()
	warehouses, err := client.Warehouses.ListAll(ctx, sql.ListWarehousesRequest{})
	if err != nil {
		return fmt.Errorf("failed to list warehouses: %w", err)
	}

	for _, wh := range warehouses {
		indicator := " "
		if wh.State == sql.StateRunning {
			indicator = "*"
		}
		fmt.Printf("%s %-20s %-40s %s\n", indicator, wh.Id, wh.Name, wh.State)
	}
	return nil
}

// LoginCmd authenticates with Databricks
type LoginCmd struct{}

func (c *LoginCmd) Run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	profileName, err := getProfileName(cfg)
	if err != nil {
		return err
	}
	profile, err := getProfile(cfg, profileName)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Authenticating to %s ...\n", profile.Host)

	ctx := context.Background()

	arg, err := u2m.NewBasicWorkspaceOAuthArgument(profile.Host)
	if err != nil {
		return fmt.Errorf("invalid host: %w", err)
	}

	auth, err := u2m.NewPersistentAuth(ctx, u2m.WithOAuthArgument(arg))
	if err != nil {
		return fmt.Errorf("failed to create auth: %w", err)
	}

	if err := auth.Challenge(); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Verify by fetching current user
	client, err := newWorkspaceClient(profile)
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

func main() {
	ctx := kong.Parse(&CLI,
		kong.Name("dbq"),
		kong.Description("Databricks SQL query tool"),
		kong.UsageOnError(),
	)
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
