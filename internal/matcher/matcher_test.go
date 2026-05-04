package matcher

import (
	"testing"
	"time"

	"github.com/fmaterak/logpipe/internal/parser"
)

func TestParseSpec_Valid(t *testing.T) {
	s, err := ParseSpec("pattern=OOM,threshold=3,window=10s")
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	if s.Pattern != "OOM" || s.Threshold != 3 || s.Window != 10*time.Second {
		t.Errorf("unexpected spec: %+v", s)
	}
}

func TestParseSpec_PatternWithEscapedComma(t *testing.T) {
	s, err := ParseSpec(`pattern=a\,b,threshold=1,window=1s`)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	if s.Pattern != "a,b" {
		t.Errorf("escaped comma: got %q", s.Pattern)
	}
}

func TestParseSpec_Errors(t *testing.T) {
	bad := []string{
		"",
		"pattern=x",
		"threshold=3,window=1s",
		"pattern=x,threshold=0,window=1s",
		"pattern=x,threshold=1,window=-1s",
		"pattern=x,threshold=abc,window=1s",
		"pattern=x,threshold=1,window=oops",
		"pattern=x,threshold=1,window=1s,unknown=y",
	}
	for _, in := range bad {
		if _, err := ParseSpec(in); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}

func TestMatcher_FiresOnThreshold(t *testing.T) {
	m, err := New([]Spec{{Pattern: "OOM", Threshold: 3, Window: 10 * time.Second}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	now := time.Now()
	first := m.Match(parser.LogEntry{Raw: "OOM event 1", Timestamp: now})
	second := m.Match(parser.LogEntry{Raw: "OOM event 2", Timestamp: now.Add(time.Second)})
	third := m.Match(parser.LogEntry{Raw: "OOM event 3", Timestamp: now.Add(2 * time.Second)})

	if len(first) != 0 || len(second) != 0 {
		t.Errorf("expected no alert before threshold, got %v %v", first, second)
	}
	if len(third) != 1 {
		t.Fatalf("expected one alert at threshold, got %v", third)
	}
	if third[0].Count != 3 {
		t.Errorf("alert count: got %d want 3", third[0].Count)
	}
}

func TestMatcher_DoesNotFireOutsideWindow(t *testing.T) {
	m, _ := New([]Spec{{Pattern: "OOM", Threshold: 3, Window: 5 * time.Second}})
	base := time.Now()
	m.Match(parser.LogEntry{Raw: "OOM 1", Timestamp: base})
	m.Match(parser.LogEntry{Raw: "OOM 2", Timestamp: base.Add(1 * time.Second)})
	got := m.Match(parser.LogEntry{Raw: "OOM 3", Timestamp: base.Add(20 * time.Second)})
	if len(got) != 0 {
		t.Errorf("expected no alert when oldest match is outside window, got %v", got)
	}
}

func TestMatcher_DebouncesRepeatedFiring(t *testing.T) {
	m, _ := New([]Spec{{Pattern: "X", Threshold: 2, Window: 5 * time.Second}})
	base := time.Now()
	m.Match(parser.LogEntry{Raw: "X", Timestamp: base})
	first := m.Match(parser.LogEntry{Raw: "X", Timestamp: base.Add(time.Second)})
	if len(first) != 1 {
		t.Fatalf("expected first fire, got %v", first)
	}

	// Second burst within the same window must be suppressed.
	repeat := m.Match(parser.LogEntry{Raw: "X", Timestamp: base.Add(2 * time.Second)})
	if len(repeat) != 0 {
		t.Errorf("expected debounce, got %v", repeat)
	}

	// Once the previous fire ages out we should be allowed to fire again,
	// but the new burst still needs `threshold` events inside the window.
	m.Match(parser.LogEntry{Raw: "X", Timestamp: base.Add(20 * time.Second)})
	again := m.Match(parser.LogEntry{Raw: "X", Timestamp: base.Add(20*time.Second + 100*time.Millisecond)})
	if len(again) != 1 {
		t.Errorf("expected refire after window expired, got %v", again)
	}
}

func TestMatcher_IgnoresNonMatchingPatterns(t *testing.T) {
	m, _ := New([]Spec{{Pattern: "OOM", Threshold: 1, Window: time.Second}})
	got := m.Match(parser.LogEntry{Raw: "everything fine", Timestamp: time.Now()})
	if len(got) != 0 {
		t.Errorf("expected no alert for non-matching line, got %v", got)
	}
}
