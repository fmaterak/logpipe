package filter

import (
	"testing"

	"github.com/fmaterak/logpipe/internal/parser"
)

func TestFieldFilter_OnParsedFields(t *testing.T) {
	f, err := FromSpec("level=error")
	if err != nil {
		t.Fatalf("FromSpec: %v", err)
	}
	cases := []struct {
		fields map[string]string
		want   bool
	}{
		{map[string]string{"level": "error"}, true},
		{map[string]string{"level": "ERROR"}, true},
		{map[string]string{"level": "info"}, false},
		{map[string]string{"other": "error"}, false},
		{nil, false},
	}
	for _, c := range cases {
		got := f.Match(parser.LogEntry{Fields: c.fields})
		if got != c.want {
			t.Errorf("fields=%v want=%v got=%v", c.fields, c.want, got)
		}
	}
}

func TestFieldFilter_FallbackOnRaw(t *testing.T) {
	f, _ := FromSpec("level=error")
	if !f.Match(parser.LogEntry{Raw: "ts=10 LEVEL=ERROR msg=boom"}) {
		t.Errorf("expected raw fallback to match case-insensitively")
	}
}

func TestRegexFilter(t *testing.T) {
	f, err := FromSpec(`regex:pattern=timeout|connection refused`)
	if err != nil {
		t.Fatalf("FromSpec: %v", err)
	}
	if !f.Match(parser.LogEntry{Raw: "got timeout from peer"}) {
		t.Errorf("expected match")
	}
	if f.Match(parser.LogEntry{Raw: "all good"}) {
		t.Errorf("expected no match")
	}
}

func TestChain_AndSemantics(t *testing.T) {
	a, _ := FromSpec("level=error")
	b, _ := FromSpec("regex:pattern=boom")
	c := Chain{Filters: []Filter{a, b}}

	if !c.Match(parser.LogEntry{Fields: map[string]string{"level": "error"}, Raw: "boom"}) {
		t.Errorf("expected match when both filters pass")
	}
	if c.Match(parser.LogEntry{Fields: map[string]string{"level": "error"}, Raw: "fine"}) {
		t.Errorf("expected no match when regex fails")
	}
	if c.Match(parser.LogEntry{Fields: map[string]string{"level": "info"}, Raw: "boom"}) {
		t.Errorf("expected no match when field fails")
	}
}

func TestChain_EmptyAcceptsAll(t *testing.T) {
	c := Chain{}
	if !c.Match(parser.LogEntry{}) {
		t.Errorf("empty chain should accept all entries")
	}
}

func TestFromSpec_UnknownFilter(t *testing.T) {
	if _, err := FromSpec("nope:k=v"); err == nil {
		t.Errorf("expected error for unknown filter")
	}
}
