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
	Output string `short:"o" help:"Output file; format is inferred from extension"`
	Format string `short:"f" default:"" help:"Output format (jsonl, json, csv, parquet, arrow, arrows)"`
}

// resolveFormat returns the effective output format.
func (o *OutputOptions) resolveFormat() (string, error) {
	if o.Format != "" && o.Output != "" {
		return "", fmt.Errorf("--format and --output are mutually exclusive")
	}
	if o.Format != "" {
		return o.Format, nil
	}
	if o.Output != "" {
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(o.Output)), ".")
		if ext == "" {
			return "", fmt.Errorf("cannot infer format from %q; use --format instead", o.Output)
		}
		return ext, nil
	}
	return "jsonl", nil
}

// WriteResult writes query results to the configured output destination.
func (o *OutputOptions) WriteResult(qr result.QueryResult) (int, error) {
	format, err := o.resolveFormat()
	if err != nil {
		return 0, err
	}

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
