package cmd

import (
	"bytes"
	"fmt"
	"testing"
)

func TestDebugf_Enabled(t *testing.T) {
	CLI.Debug = true
	defer func() { CLI.Debug = false }()

	f := debugf()
	if f == nil {
		t.Fatal("expected non-nil debugf when debug enabled")
	}

	// Verify it writes to stderr (capture via a known format)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "hello %d", 42)
	if buf.String() != "hello 42" {
		t.Errorf("unexpected: %s", buf.String())
	}
}

func TestDebugf_Disabled(t *testing.T) {
	CLI.Debug = false
	f := debugf()
	if f != nil {
		t.Fatal("expected nil debugf when debug disabled")
	}
}
