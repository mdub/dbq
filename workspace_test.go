package main

import (
	"testing"
)

func TestGetWorkspaceHost_SimpleName(t *testing.T) {
	CLI.Workspace = "my-workspace"
	host, err := getWorkspaceHost()
	if err != nil {
		t.Fatal(err)
	}
	if host != "https://my-workspace.cloud.databricks.com" {
		t.Errorf("got %s", host)
	}
}

func TestGetWorkspaceHost_FullHostname(t *testing.T) {
	CLI.Workspace = "my-workspace.cloud.databricks.com"
	host, err := getWorkspaceHost()
	if err != nil {
		t.Fatal(err)
	}
	if host != "https://my-workspace.cloud.databricks.com" {
		t.Errorf("got %s", host)
	}
}

func TestGetWorkspaceHost_FullURL(t *testing.T) {
	CLI.Workspace = "https://my-workspace.cloud.databricks.com"
	host, err := getWorkspaceHost()
	if err != nil {
		t.Fatal(err)
	}
	if host != "https://my-workspace.cloud.databricks.com" {
		t.Errorf("got %s", host)
	}
}

func TestGetWorkspaceHost_TrailingSlash(t *testing.T) {
	CLI.Workspace = "https://my-workspace.cloud.databricks.com/"
	host, err := getWorkspaceHost()
	if err != nil {
		t.Fatal(err)
	}
	if host != "https://my-workspace.cloud.databricks.com" {
		t.Errorf("got %s", host)
	}
}

func TestGetWorkspaceHost_Empty(t *testing.T) {
	CLI.Workspace = ""
	_, err := getWorkspaceHost()
	if err == nil {
		t.Fatal("expected error for empty host")
	}
}

func TestIsWarehouseID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"abcdef0123456789", true},
		{"ABCDEF0123456789", true},
		{"abcdef01234567890abc", true},
		{"abc123", false},          // too short
		{"abcdef012345678g", false}, // non-hex character
		{"my-warehouse", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isWarehouseID(tt.input); got != tt.want {
			t.Errorf("isWarehouseID(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
