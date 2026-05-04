package plugins

import (
	"testing"

	"github.com/fmaterak/logpipe/internal/filter"
	"github.com/fmaterak/logpipe/internal/parser"
)

func TestThresholdFilter_AllOps(t *testing.T) {
	cases := []struct {
		op    string
		val   string
		field string
		want  bool
	}{
		{"gt", "100", "150", true},
		{"gt", "100", "100", false},
		{"gte", "100", "100", true},
		{"lt", "100", "50", true},
		{"lte", "100", "100", true},
		{"eq", "100", "100", true},
		{"eq", "100", "99", false},
	}
	for _, c := range cases {
		f, err := filter.FromSpec("threshold:field=latency_ms,op=" + c.op + ",value=" + c.val)
		if err != nil {
			t.Fatalf("FromSpec(%s,%s): %v", c.op, c.val, err)
		}
		entry := parser.LogEntry{Fields: map[string]string{"latency_ms": c.field}}
		if got := f.Match(entry); got != c.want {
			t.Errorf("op=%s val=%s field=%s: got %v want %v", c.op, c.val, c.field, got, c.want)
		}
	}
}

func TestThresholdFilter_RejectsNonNumeric(t *testing.T) {
	f, err := filter.FromSpec("threshold:field=lat,op=gt,value=10")
	if err != nil {
		t.Fatalf("FromSpec: %v", err)
	}
	entry := parser.LogEntry{Fields: map[string]string{"lat": "fast"}}
	if f.Match(entry) {
		t.Errorf("expected non-numeric value to be rejected")
	}
}

func TestThresholdFilter_RegistrationErrors(t *testing.T) {
	bad := []string{
		"threshold:op=gt,value=10",
		"threshold:field=lat,value=10",
		"threshold:field=lat,op=??,value=10",
		"threshold:field=lat,op=gt,value=oops",
	}
	for _, s := range bad {
		if _, err := filter.FromSpec(s); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}
