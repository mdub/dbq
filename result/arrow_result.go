package result

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"net/http"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/databricks/databricks-sdk-go"
	"github.com/databricks/databricks-sdk-go/service/sql"
)

// NewArrowResult creates a QueryResult from a Databricks API response
// that uses ARROW_STREAM format with EXTERNAL_LINKS disposition.
// The debugf function, if non-nil, is called with debug messages.
func NewArrowResult(ctx context.Context, client *databricks.WorkspaceClient, response *sql.StatementResponse, debugf func(string, ...any)) QueryResult {
	return &arrowResult{
		ctx:      ctx,
		client:   client,
		response: response,
		debugf:   debugf,
	}
}

// arrowResult implements QueryResult by fetching Arrow IPC data
// from external links provided by the Databricks API.
type arrowResult struct {
	ctx      context.Context
	client   *databricks.WorkspaceClient
	response *sql.StatementResponse
	debugf   func(string, ...any)
}

func (r *arrowResult) StatementID() string {
	return r.response.StatementId
}

func (r *arrowResult) Chunks() iter.Seq2[arrow.RecordBatch, error] {
	return func(yield func(arrow.RecordBatch, error) bool) {
		if r.response.Result == nil {
			return
		}

		// Process external links from the initial response
		for _, link := range r.response.Result.ExternalLinks {
			if !r.yieldRecordsFromLink(link, yield) {
				return
			}
		}

		// Fetch additional chunks if indicated
		nextChunk := r.response.Result.NextChunkIndex
		for nextChunk > 0 {
			if r.debugf != nil {
				r.debugf("fetching chunk %d", nextChunk)
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
				if !r.yieldRecordsFromLink(link, yield) {
					return
				}
			}
			nextChunk = chunk.NextChunkIndex
		}
	}
}

// yieldRecordsFromLink downloads Arrow IPC data from an external link
// and yields each record batch. Returns false if iteration should stop.
func (r *arrowResult) yieldRecordsFromLink(link sql.ExternalLink, yield func(arrow.RecordBatch, error) bool) bool {
	if link.ExternalLink == "" {
		return true
	}

	data, err := fetchExternalLink(r.ctx, link)
	if err != nil {
		return yield(nil, fmt.Errorf("fetching chunk %d: %w", link.ChunkIndex, err))
	}

	for rec, err := range readArrowStream(bytes.NewReader(data)) {
		if err != nil {
			return yield(nil, fmt.Errorf("reading Arrow data for chunk %d: %w", link.ChunkIndex, err))
		}
		if !yield(rec, nil) {
			return false
		}
	}
	return true
}

// readArrowStream reads an Arrow IPC stream and yields record batches.
func readArrowStream(r io.Reader) iter.Seq2[arrow.RecordBatch, error] {
	return func(yield func(arrow.RecordBatch, error) bool) {
		reader, err := ipc.NewReader(r)
		if err != nil {
			yield(nil, fmt.Errorf("reading Arrow stream: %w", err))
			return
		}
		defer reader.Release()

		for reader.Next() {
			if !yield(reader.RecordBatch(), nil) {
				return
			}
		}
		if err := reader.Err(); err != nil {
			yield(nil, err)
		}
	}
}

// fetchExternalLink downloads data from a Databricks external link,
// including any required HTTP headers.
func fetchExternalLink(ctx context.Context, link sql.ExternalLink) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", link.ExternalLink, nil)
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
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching external link", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
