// Package result defines the QueryResult interface and its Databricks Arrow implementation.
package result

import (
	"iter"

	"github.com/apache/arrow-go/v18/arrow"
)

// QueryResult provides access to query result data.
type QueryResult interface {
	StatementID() string
	Chunks() iter.Seq2[arrow.RecordBatch, error]
}
