// Package cmd implements the dbq CLI commands.
package cmd

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

var CLI struct {
	Workspace   string `env:"DBQ_WORKSPACE" help:"Databricks workspace"`
	Warehouse   string `env:"DBQ_WAREHOUSE" help:"SQL warehouse ID or name"`
	AutoLogin   bool   `help:"Force auto re-authentication, even when stderr is not a terminal"`
	NoAutoLogin bool   `help:"Disable auto re-authentication, even when stderr is a terminal"`
	Debug       bool   `help:"Enable debug output"`

	SQL        SQLCmd        `cmd:"" help:"Execute SQL query"`
	Query      QueryCmd      `cmd:"" help:"Manage async queries"`
	Warehouses WarehousesCmd `cmd:"" help:"List SQL warehouses"`
	Auth       AuthCmd       `cmd:"" help:"Authentication commands"`
	Cheatsheet CheatsheetCmd `cmd:"" help:"Print usage cheatsheet"`
}

// logDebug logs a formatted message to stderr when debug mode is enabled.
func logDebug(format string, args ...any) {
	if CLI.Debug {
		fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
	}
}

func Run() {
	ctx := kong.Parse(&CLI,
		kong.Name("dbq"),
		kong.Description("Databricks SQL query tool"),
		kong.UsageOnError(),
	)
	if CLI.AutoLogin && CLI.NoAutoLogin {
		ctx.Fatalf("cannot specify both --auto-login and --no-auto-login")
	}
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
