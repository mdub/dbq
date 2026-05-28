package cmd

import (
	"testing"

	"github.com/databricks/databricks-sdk-go/service/sql"
)

func TestStatementStatusSummary(t *testing.T) {
	tests := []struct {
		name   string
		status sql.StatementStatus
		want   string
	}{
		{
			name:   "state only",
			status: sql.StatementStatus{State: sql.StatementStateFailed},
			want:   "FAILED",
		},
		{
			name: "state and sqlstate",
			status: sql.StatementStatus{
				State:    sql.StatementStateFailed,
				SqlState: "42P01",
			},
			want: "FAILED (SQLSTATE 42P01)",
		},
		{
			name: "state, sqlstate, and message",
			status: sql.StatementStatus{
				State:    sql.StatementStateFailed,
				SqlState: "42P01",
				Error:    &sql.ServiceError{Message: "no such table"},
			},
			want: "FAILED (SQLSTATE 42P01)\nno such table",
		},
		{
			name: "state and message, no sqlstate",
			status: sql.StatementStatus{
				State: sql.StatementStateFailed,
				Error: &sql.ServiceError{Message: "boom"},
			},
			want: "FAILED\nboom",
		},
		{
			name:   "succeeded",
			status: sql.StatementStatus{State: sql.StatementStateSucceeded},
			want:   "SUCCEEDED",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statementStatusSummary(&tt.status)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseID(t *testing.T) {
	id, err := parseID("12345", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 12345 {
		t.Errorf("got %d, want 12345", id)
	}
	if _, err := parseID("not-a-number", "user"); err == nil {
		t.Errorf("expected error for non-numeric input")
	}
}
