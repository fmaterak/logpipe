package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/fmaterak/logpipe/internal/parser"
)

func mustGet(t *testing.T, name string, w *bytes.Buffer) Outputter {
	t.Helper()
	o, err := Get(name, w)
	if err != nil {
		t.Fatalf("Get(%q): %v", name, err)
	}
	return o
}

func TestText_AppendsNewline(t *testing.T) {
	var buf bytes.Buffer
	o := mustGet(t, "text", &buf)
	if err := o.Write(parser.LogEntry{Raw: "no newline"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	o.Close()
	if got := buf.String(); got != "no newline\n" {
		t.Errorf("got %q", got)
	}
}

func TestText_PreservesExistingNewline(t *testing.T) {
	var buf bytes.Buffer
	o := mustGet(t, "text", &buf)
	o.Write(parser.LogEntry{Raw: "with newline\n"})
	o.Close()
	if got := buf.String(); got != "with newline\n" {
		t.Errorf("got %q", got)
	}
}

func TestJSON_NDJSONLines(t *testing.T) {
	var buf bytes.Buffer
	o := mustGet(t, "json", &buf)
	ts := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	o.Write(parser.LogEntry{Raw: "r1", Timestamp: ts, Fields: map[string]string{"level": "info"}})
	o.Write(parser.LogEntry{Raw: "r2", Timestamp: ts})
	o.Close()

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d: %q", len(lines), buf.String())
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if first["_raw"] != "r1" || first["level"] != "info" {
		t.Errorf("unexpected first line: %v", first)
	}
}

func TestCSV_HeaderAndRows(t *testing.T) {
	var buf bytes.Buffer
	o := mustGet(t, "csv", &buf)
	ts := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	o.Write(parser.LogEntry{Raw: "r1", Timestamp: ts, Fields: map[string]string{"level": "info"}})
	o.Write(parser.LogEntry{Raw: "r2", Timestamp: ts, Fields: map[string]string{"level": "error"}})
	o.Close()

	r := csv.NewReader(&buf)
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(rows) < 3 {
		t.Fatalf("expected header + 2 rows, got %d: %v", len(rows), rows)
	}
	if rows[0][0] != "_timestamp" || rows[0][1] != "_raw" || rows[0][2] != "level" {
		t.Errorf("unexpected header: %v", rows[0])
	}
}
