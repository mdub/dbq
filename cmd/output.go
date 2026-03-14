package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mdub/dbq/output"
	"github.com/mdub/dbq/result"
)

// OutputOptions provides shared flags for output destination and format.
type OutputOptions struct {
	Output string `short:"o" help:"Output file (default: stdout)"`
	Format string `short:"f" default:"" help:"Output format (jsonl, json, csv, parquet, arrow, arrows)"`
}

// resolveFormat returns the effective output format.
// Explicit --format wins, then infer from file extension, then default to "jsonl".
func (o *OutputOptions) resolveFormat() string {
	if o.Format != "" {
		return o.Format
	}
	if o.Output != "" {
		switch strings.ToLower(filepath.Ext(o.Output)) {
		case ".parquet":
			return "parquet"
		case ".csv":
			return "csv"
		case ".jsonl":
			return "jsonl"
		case ".json":
			return "json"
		case ".arrows":
			return "arrows"
		case ".arrow", ".feather", ".ipc":
			return "arrow"
		}
	}
	return "jsonl"
}

// WriteResult writes query results to the configured output destination.
func (o *OutputOptions) WriteResult(qr result.QueryResult) (int, error) {
	format := o.resolveFormat()

	if o.Output == "" {
		return output.WriteResult(os.Stdout, qr, format)
	}

	f, err := os.Create(o.Output)
	if err != nil {
		return 0, fmt.Errorf("failed to open output file: %w", err)
	}
	n, err := output.WriteResult(f, qr, format)
	if err != nil {
		f.Close() //nolint:errcheck
		return n, err
	}
	// Some formatters (e.g. parquet) close the underlying writer,
	// so we ignore "already closed" errors here.
	if err := f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return n, err
	}
	return n, nil
}
