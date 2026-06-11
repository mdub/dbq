package cmd

import (
	"testing"

	"github.com/databricks/databricks-sdk-go/service/sql"
)

func TestExecuteWaitParams(t *testing.T) {
	if wt, owt := executeWaitParams(true, 30); wt != "0s" || owt != sql.ExecuteStatementRequestOnWaitTimeoutContinue {
		t.Errorf("async: got (%q, %v), want (\"0s\", CONTINUE)", wt, owt)
	}
	if wt, owt := executeWaitParams(false, 30); wt != "30s" || owt != sql.ExecuteStatementRequestOnWaitTimeoutCancel {
		t.Errorf("sync: got (%q, %v), want (\"30s\", CANCEL)", wt, owt)
	}
}

func TestIndentSQL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			"SELECT 1",
			"  SELECT 1",
		},
		{
			"\n  SELECT\n    *\n  FROM t\n\n",
			"    SELECT\n      *\n    FROM t",
		},
		{
			"",
			"",
		},
	}
	for _, tt := range tests {
		got := indentSQL(tt.input)
		if got != tt.want {
			t.Errorf("indentSQL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
