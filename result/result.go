// Package result defines the QueryResult interface for accessing query data.
package result

import "iter"

// QueryResult provides access to query result data.
type QueryResult interface {
	StatementID() string
	ColumnNames() []string
	Chunks() iter.Seq2[[]map[string]any, error]
}
