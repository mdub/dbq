package cmd

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/databricks/databricks-sdk-go/service/sql"
)

// fakeExecutor scripts a sequence of statement states for GetStatement and
// records whether CancelExecution was called.
type fakeExecutor struct {
	states    []sql.StatementState
	calls     int
	canceled  bool
	cancelErr error
}

func (f *fakeExecutor) GetStatement(_ context.Context, req sql.GetStatementRequest) (*sql.StatementResponse, error) {
	i := f.calls
	if i >= len(f.states) {
		i = len(f.states) - 1
	}
	f.calls++
	return &sql.StatementResponse{
		StatementId: req.StatementId,
		Status:      &sql.StatementStatus{State: f.states[i]},
	}, nil
}

func (f *fakeExecutor) CancelExecution(_ context.Context, _ sql.CancelExecutionRequest) error {
	f.canceled = true
	return f.cancelErr
}

func TestPollUntilTerminal(t *testing.T) {
	ctx := context.Background()
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Second)
	fast := time.Millisecond

	t.Run("succeeds", func(t *testing.T) {
		f := &fakeExecutor{states: []sql.StatementState{sql.StatementStateSucceeded}}
		resp, err := pollUntilTerminal(ctx, f, "s1", future, 30, fast, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.StatementId != "s1" {
			t.Errorf("got statement id %q", resp.StatementId)
		}
	})

	t.Run("polls running then succeeds", func(t *testing.T) {
		f := &fakeExecutor{states: []sql.StatementState{sql.StatementStateRunning, sql.StatementStateSucceeded}}
		if _, err := pollUntilTerminal(ctx, f, "s1", future, 30, fast, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.calls < 2 {
			t.Errorf("expected at least 2 polls, got %d", f.calls)
		}
	})

	t.Run("failed", func(t *testing.T) {
		f := &fakeExecutor{states: []sql.StatementState{sql.StatementStateFailed}}
		if _, err := pollUntilTerminal(ctx, f, "s1", future, 30, fast, true); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("canceled state", func(t *testing.T) {
		f := &fakeExecutor{states: []sql.StatementState{sql.StatementStateCanceled}}
		_, err := pollUntilTerminal(ctx, f, "s1", future, 30, fast, true)
		if err == nil || !strings.Contains(err.Error(), "canceled") {
			t.Fatalf("got %v, want canceled error", err)
		}
	})

	t.Run("timeout cancels", func(t *testing.T) {
		f := &fakeExecutor{states: []sql.StatementState{sql.StatementStateRunning}}
		_, err := pollUntilTerminal(ctx, f, "s1", past, 30, fast, true)
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("got %v, want timeout error", err)
		}
		if !f.canceled {
			t.Error("expected statement to be cancelled on timeout")
		}
	})

	t.Run("timeout without cancel leaves statement running", func(t *testing.T) {
		f := &fakeExecutor{states: []sql.StatementState{sql.StatementStateRunning}}
		_, err := pollUntilTerminal(ctx, f, "s1", past, 30, fast, false)
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("got %v, want timeout error", err)
		}
		if f.canceled {
			t.Error("did not expect cancellation")
		}
	})
}

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
