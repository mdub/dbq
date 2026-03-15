package metrics

import (
	"testing"

	"github.com/databricks/databricks-sdk-go/service/sql"
)

func TestBuild(t *testing.T) {
	m := &sql.QueryMetrics{
		CompilationTimeMs: 120,
		ExecutionTimeMs:   3450,
		ResultFetchTimeMs: 200,
		ReadBytes:         1073741824,
		ReadCacheBytes:    536870912,
		ReadFilesCount:    42,
		RowsReadCount:     15000000,
		RowsProducedCount: 1000,
		ResultFromCache:   false,
	}

	qm := Build(m)

	if qm.Time.CompilationMs != 120 {
		t.Errorf("CompilationMs = %d, want 120", qm.Time.CompilationMs)
	}
	if qm.Time.ExecutionMs != 3450 {
		t.Errorf("ExecutionMs = %d, want 3450", qm.Time.ExecutionMs)
	}
	if qm.Time.FetchMs != 200 {
		t.Errorf("FetchMs = %d, want 200", qm.Time.FetchMs)
	}
	if qm.Time.TotalMs != 3770 {
		t.Errorf("TotalMs = %d, want 3770", qm.Time.TotalMs)
	}
	if qm.Cached {
		t.Error("Cached = true, want false")
	}
	if qm.Read == nil {
		t.Fatal("Read is nil, want non-nil")
	}
	if qm.Read.Bytes != 1073741824 {
		t.Errorf("Read.Bytes = %d, want 1073741824", qm.Read.Bytes)
	}
	if qm.Read.CacheBytes != 536870912 {
		t.Errorf("Read.CacheBytes = %d, want 536870912", qm.Read.CacheBytes)
	}
	if qm.Read.Files != 42 {
		t.Errorf("Read.Files = %d, want 42", qm.Read.Files)
	}
	if qm.Read.Rows != 15000000 {
		t.Errorf("Read.Rows = %d, want 15000000", qm.Read.Rows)
	}
	if qm.Produced == nil {
		t.Fatal("Produced is nil, want non-nil")
	}
	if qm.Produced.Rows != 1000 {
		t.Errorf("Produced.Rows = %d, want 1000", qm.Produced.Rows)
	}
}

func TestBuild_cached(t *testing.T) {
	m := &sql.QueryMetrics{
		CompilationTimeMs: 50,
		ExecutionTimeMs:   0,
		ResultFromCache:   true,
	}
	qm := Build(m)
	if !qm.Cached {
		t.Error("Cached = false, want true")
	}
}

func TestBuild_noReadOrProduced(t *testing.T) {
	m := &sql.QueryMetrics{
		CompilationTimeMs: 100,
		ExecutionTimeMs:   200,
	}
	qm := Build(m)
	if qm.Read != nil {
		t.Errorf("Read = %+v, want nil", qm.Read)
	}
	if qm.Produced != nil {
		t.Errorf("Produced = %+v, want nil", qm.Produced)
	}
}
