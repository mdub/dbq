package cmd

import "testing"

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
