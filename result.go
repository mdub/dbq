package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"net/http"
	"os"

	"github.com/databricks/databricks-sdk-go"
	"github.com/databricks/databricks-sdk-go/service/sql"
)

// QueryResult provides access to query result data.
type QueryResult interface {
	StatementID() string
	ColumnNames() []string
	Chunks() iter.Seq2[[]map[string]any, error]
}

// newQueryResult creates a QueryResult from a Databricks API response
// that uses ARROW_STREAM format with EXTERNAL_LINKS disposition.
func newQueryResult(ctx context.Context, client *databricks.WorkspaceClient, response *sql.StatementResponse) QueryResult {
	return &arrowResult{
		ctx:      ctx,
		client:   client,
		response: response,
	}
}

// arrowResult implements QueryResult by fetching Arrow IPC data
// from external links provided by the Databricks API.
type arrowResult struct {
	ctx         context.Context
	client      *databricks.WorkspaceClient
	response    *sql.StatementResponse
	columnNames []string // populated on first chunk fetch
}

func (r *arrowResult) StatementID() string {
	return r.response.StatementId
}

func (r *arrowResult) ColumnNames() []string {
	if r.columnNames != nil {
		return r.columnNames
	}
	// Fall back to manifest schema if we haven't fetched any Arrow data yet
	if r.response.Manifest != nil && r.response.Manifest.Schema != nil {
		names := make([]string, len(r.response.Manifest.Schema.Columns))
		for i, col := range r.response.Manifest.Schema.Columns {
			names[i] = col.Name
		}
		return names
	}
	return nil
}

func (r *arrowResult) Chunks() iter.Seq2[[]map[string]any, error] {
	return func(yield func([]map[string]any, error) bool) {
		if r.response.Result == nil {
			return
		}

		// Process external links from the initial response
		for _, link := range r.response.Result.ExternalLinks {
			rows, err := r.fetchArrowChunk(link)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(rows, nil) {
				return
			}
		}

		// Fetch additional chunks if indicated
		nextChunk := r.response.Result.NextChunkIndex
		for nextChunk > 0 {
			if CLI.Debug {
				fmt.Fprintf(os.Stderr, "DEBUG: fetching chunk %d\n", nextChunk)
			}
			chunk, err := r.client.StatementExecution.GetStatementResultChunkN(r.ctx, sql.GetStatementResultChunkNRequest{
				StatementId: r.response.StatementId,
				ChunkIndex:  nextChunk,
			})
			if err != nil {
				yield(nil, fmt.Errorf("failed to fetch chunk %d: %w", nextChunk, err))
				return
			}
			for _, link := range chunk.ExternalLinks {
				rows, err := r.fetchArrowChunk(link)
				if err != nil {
					yield(nil, err)
					return
				}
				if !yield(rows, nil) {
					return
				}
			}
			nextChunk = chunk.NextChunkIndex
		}
	}
}

// fetchArrowChunk downloads Arrow IPC data from an external link
// and converts it to typed maps.
func (r *arrowResult) fetchArrowChunk(link sql.ExternalLink) ([]map[string]any, error) {
	if link.ExternalLink == "" {
		return nil, nil
	}

	data, err := fetchExternalLink(link)
	if err != nil {
		return nil, fmt.Errorf("fetching chunk %d: %w", link.ChunkIndex, err)
	}

	columnNames, rows, err := readArrowStream(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("reading Arrow data for chunk %d: %w", link.ChunkIndex, err)
	}

	// Capture column names from the first chunk
	if r.columnNames == nil && len(columnNames) > 0 {
		r.columnNames = columnNames
	}

	return rows, nil
}

// fetchExternalLink downloads data from a Databricks external link,
// including any required HTTP headers.
func fetchExternalLink(link sql.ExternalLink) ([]byte, error) {
	req, err := http.NewRequest("GET", link.ExternalLink, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range link.HttpHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching external link", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
