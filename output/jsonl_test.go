package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONLFormatter(t *testing.T) {
	rows := []map[string]interface{}{
		{"name": "Alice", "age": "30"},
		{"name": "Bob", "age": "25"},
	}
	var buf bytes.Buffer
	f := newJSONLFormatter(&buf)
	if err := f.Start([]string{"name", "age"}); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteChunk(rows); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}
	for i, line := range lines {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Fatalf("line %d: invalid JSON: %v", i, err)
		}
	}
}

func TestJSONLFormatter_Empty(t *testing.T) {
	var buf bytes.Buffer
	f := newJSONLFormatter(&buf)
	if err := f.Start(nil); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "" {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

func TestJSONLFormatter_StructuredValues(t *testing.T) {
	rows := []map[string]interface{}{
		{
			"name": "Alice",
			"tags": []interface{}{"a", "b"},
			"meta": map[string]interface{}{"role": "admin"},
		},
	}
	var buf bytes.Buffer
	f := newJSONLFormatter(&buf)
	if err := f.Start([]string{"name", "tags", "meta"}); err != nil {
		t.Fatal(err)
	}
	if err := f.WriteChunk(rows); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
	tags, ok := parsed["tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Errorf("tags = %v, want [a b]", parsed["tags"])
	}
	meta, ok := parsed["meta"].(map[string]interface{})
	if !ok || meta["role"] != "admin" {
		t.Errorf("meta = %v, want {role: admin}", parsed["meta"])
	}
}
