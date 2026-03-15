// Package metrics provides query execution metrics from the Databricks Query
// History API.
package metrics

import (
	"context"
	"fmt"

	"github.com/databricks/databricks-sdk-go"
	"github.com/databricks/databricks-sdk-go/service/sql"
)

// QueryMetrics is a structured summary of query execution metrics.
type QueryMetrics struct {
	Time     TimeMetrics      `json:"time"`
	Cached   bool             `json:"cached"`
	Read     *ReadMetrics     `json:"read,omitempty"`
	Pruned   *PrunedMetrics   `json:"pruned,omitempty"`
	Produced *ProducedMetrics `json:"produced,omitempty"`
}

type TimeMetrics struct {
	CompilationMs int64 `json:"compilation_ms"`
	ExecutionMs   int64 `json:"execution_ms"`
	FetchMs       int64 `json:"fetch_ms,omitempty"`
	TotalMs       int64 `json:"total_ms,omitempty"`
}

type ReadMetrics struct {
	Bytes      int64 `json:"bytes,omitempty"`
	CacheBytes int64 `json:"cache_bytes,omitempty"`
	Files      int64 `json:"files,omitempty"`
	Rows       int64 `json:"rows,omitempty"`
}

type PrunedMetrics struct {
	Bytes int64 `json:"bytes,omitempty"`
	Files int64 `json:"files,omitempty"`
}

type ProducedMetrics struct {
	Rows int64 `json:"rows,omitempty"`
}

// Fetch retrieves execution metrics for a statement from the Databricks Query
// History API.
func Fetch(ctx context.Context, client *databricks.WorkspaceClient, statementID string) (*QueryMetrics, error) {
	resp, err := client.QueryHistory.List(ctx, sql.ListQueryHistoryRequest{
		FilterBy: &sql.QueryFilter{
			StatementIds: []string{statementID},
		},
		IncludeMetrics: true,
		MaxResults:     1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch query metrics: %w", err)
	}
	if len(resp.Res) == 0 || resp.Res[0].Metrics == nil {
		return nil, fmt.Errorf("no query metrics available")
	}

	return Build(resp.Res[0].Metrics), nil
}

// Build maps Databricks SDK metrics to our structured summary.
func Build(m *sql.QueryMetrics) *QueryMetrics {
	qm := &QueryMetrics{
		Time: TimeMetrics{
			CompilationMs: m.CompilationTimeMs,
			ExecutionMs:   m.ExecutionTimeMs,
			FetchMs:       m.ResultFetchTimeMs,
			TotalMs:       m.CompilationTimeMs + m.ExecutionTimeMs + m.ResultFetchTimeMs,
		},
		Cached: m.ResultFromCache,
	}
	if m.ReadBytes > 0 || m.ReadCacheBytes > 0 || m.ReadFilesCount > 0 || m.RowsReadCount > 0 {
		qm.Read = &ReadMetrics{
			Bytes:      m.ReadBytes,
			CacheBytes: m.ReadCacheBytes,
			Files:      m.ReadFilesCount,
			Rows:       m.RowsReadCount,
		}
	}
	if m.PrunedBytes > 0 || m.PrunedFilesCount > 0 {
		qm.Pruned = &PrunedMetrics{
			Bytes: m.PrunedBytes,
			Files: m.PrunedFilesCount,
		}
	}
	if m.RowsProducedCount > 0 {
		qm.Produced = &ProducedMetrics{
			Rows: m.RowsProducedCount,
		}
	}
	return qm
}
