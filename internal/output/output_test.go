package output

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestJSON_TTY_PrettyPrints(t *testing.T) {
	var buf bytes.Buffer
	r := NewTestRenderer(&buf, true, false, false, true)

	data := map[string]string{"name": "Test"}
	if err := r.JSON(data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "\n") {
		t.Error("expected pretty-printed JSON with newlines for TTY")
	}
	if !strings.Contains(out, "  ") {
		t.Error("expected indentation in TTY JSON output")
	}
}

func TestJSON_NonTTY_Compact(t *testing.T) {
	var buf bytes.Buffer
	r := NewTestRenderer(&buf, true, false, false, false)

	data := map[string]string{"name": "Test"}
	if err := r.JSON(data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Compact JSON should be a single line (with trailing newline from Encode).
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 1 {
		t.Errorf("expected single-line compact JSON, got %d lines: %q", len(lines), out)
	}
}

func TestJSON_Quiet_NoOutput(t *testing.T) {
	var buf bytes.Buffer
	r := NewTestRenderer(&buf, true, true, false, true)

	if err := r.JSON(map[string]string{"a": "b"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output in quiet mode, got %q", buf.String())
	}
}

func TestJSONError_Format(t *testing.T) {
	var buf bytes.Buffer
	r := NewTestRenderer(&buf, true, false, false, false)

	if err := r.JSONError(errors.New("something broke"), 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed struct {
		Error string `json:"error"`
		Code  int    `json:"code"`
	}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v — output was: %q", err, buf.String())
	}
	if parsed.Error != "something broke" {
		t.Errorf("expected error 'something broke', got %q", parsed.Error)
	}
	if parsed.Code != 42 {
		t.Errorf("expected code 42, got %d", parsed.Code)
	}
}

func TestKeyValue_Alignment(t *testing.T) {
	var buf bytes.Buffer
	r := NewTestRenderer(&buf, false, false, true, false) // noColor=true

	pairs := []KVPair{
		{Key: "Name", Value: "Alice"},
		{Key: "Company Number", Value: "12345"},
	}
	if err := r.KeyValue(pairs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}

	// Both values should start at the same column.
	idx1 := strings.Index(lines[0], "Alice")
	idx2 := strings.Index(lines[1], "12345")
	if idx1 != idx2 {
		t.Errorf("values not aligned: 'Alice' at col %d, '12345' at col %d", idx1, idx2)
	}
}

func TestKeyValue_Quiet_NoOutput(t *testing.T) {
	var buf bytes.Buffer
	r := NewTestRenderer(&buf, false, true, false, false)

	if err := r.KeyValue([]KVPair{{Key: "A", Value: "B"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output in quiet mode, got %q", buf.String())
	}
}

func TestTable_BasicOutput(t *testing.T) {
	var buf bytes.Buffer
	r := NewTestRenderer(&buf, false, false, true, false)

	headers := []string{"ID", "Name", "Status"}
	rows := [][]string{
		{"1", "Invoice A", "SENT"},
		{"2", "Invoice B", "DRAFT"},
	}
	if err := r.Table(headers, rows); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "STATUS", "Invoice A", "DRAFT"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q, got:\n%s", want, out)
		}
	}
}

func TestTable_Quiet_NoOutput(t *testing.T) {
	var buf bytes.Buffer
	r := NewTestRenderer(&buf, false, true, false, false)

	if err := r.Table([]string{"A"}, [][]string{{"1"}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output in quiet mode, got %q", buf.String())
	}
}

func TestPagination_Format(t *testing.T) {
	var buf bytes.Buffer
	r := NewTestRenderer(&buf, false, false, true, false)

	r.Pagination(2, 20, 55)

	out := buf.String()
	expected := "Showing 21-40 of 55 documents (page 2/3)"
	if !strings.Contains(out, expected) {
		t.Errorf("expected %q, got %q", expected, out)
	}
}

func TestPagination_LastPage(t *testing.T) {
	var buf bytes.Buffer
	r := NewTestRenderer(&buf, false, false, true, false)

	r.Pagination(3, 20, 55)

	out := buf.String()
	if !strings.Contains(out, "Showing 41-55 of 55") {
		t.Errorf("expected clamped end, got %q", out)
	}
}

func TestPagination_Zero_NoOutput(t *testing.T) {
	var buf bytes.Buffer
	r := NewTestRenderer(&buf, false, false, true, false)

	r.Pagination(1, 20, 0)

	if buf.Len() != 0 {
		t.Errorf("expected no output for zero total, got %q", buf.String())
	}
}

func TestSuccess(t *testing.T) {
	var buf bytes.Buffer
	r := NewTestRenderer(&buf, false, false, true, false)

	r.Success("done")
	if !strings.Contains(buf.String(), "done") {
		t.Errorf("expected 'done', got %q", buf.String())
	}
}

func TestError_AlwaysShown(t *testing.T) {
	var buf bytes.Buffer
	// Even in quiet mode, errors should be shown.
	r := NewTestRenderer(&buf, false, true, true, false)

	r.Error("fail")
	if !strings.Contains(buf.String(), "fail") {
		t.Errorf("expected 'fail' even in quiet mode, got %q", buf.String())
	}
}

func TestStatusBadge_Mapping(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"sent", "SENT"},
		{"DRAFT", "DRAFT"},
		{"failed", "FAILED"},
		{"unknown", "UNKNOWN"},
	}
	for _, tt := range tests {
		got := StatusBadge(tt.status)
		// Strip ANSI codes for comparison — in test env with noColor the raw string comes through.
		if !strings.Contains(got, tt.want) {
			t.Errorf("StatusBadge(%q) = %q, want to contain %q", tt.status, got, tt.want)
		}
	}
}

func TestHasColor(t *testing.T) {
	if hasColor(true, true) {
		t.Error("hasColor should be false when noColor flag is set")
	}
	if hasColor(false, false) {
		t.Error("hasColor should be false when not TTY")
	}
}

func TestFromContext_Default(t *testing.T) {
	r := FromContext(context.Background())
	if r == nil {
		t.Fatal("FromContext should return a non-nil default renderer")
	}
}

func TestFromContext_Roundtrip(t *testing.T) {
	var buf bytes.Buffer
	orig := NewTestRenderer(&buf, true, false, false, false)
	ctx := WithRenderer(context.Background(), orig)

	got := FromContext(ctx)
	if got != orig {
		t.Error("FromContext did not return the stored renderer")
	}
}

func TestFromContext_Nil(t *testing.T) {
	r := FromContext(context.TODO())
	if r == nil {
		t.Fatal("FromContext(nil) should return a non-nil default renderer")
	}
}

func TestIsJSON(t *testing.T) {
	r := NewTestRenderer(&bytes.Buffer{}, true, false, false, false)
	if !r.IsJSON() {
		t.Error("expected IsJSON to be true")
	}
}

func TestIsQuiet(t *testing.T) {
	r := NewTestRenderer(&bytes.Buffer{}, false, true, false, false)
	if !r.IsQuiet() {
		t.Error("expected IsQuiet to be true")
	}
}
