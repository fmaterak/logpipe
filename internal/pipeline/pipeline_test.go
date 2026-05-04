package pipeline

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fmaterak/logpipe/internal/filter"
	"github.com/fmaterak/logpipe/internal/matcher"
	"github.com/fmaterak/logpipe/internal/output"
	"github.com/fmaterak/logpipe/internal/parser"
	"github.com/fmaterak/logpipe/internal/source"
)

// buildPipeline composes a pipeline reading from `input` (one line per \n) and
// writing text output into the returned buffer. Used by the tests below as a
// fixture.
func buildPipeline(t *testing.T, input string, ff filter.Filter, m *matcher.Matcher) (*Pipeline, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	src := source.NewStdin(context.Background(), strings.NewReader(input))
	var stdout, stderr bytes.Buffer
	out, err := output.Get("text", &stdout)
	if err != nil {
		t.Fatalf("output: %v", err)
	}
	p, err := New().
		Source(src).
		Parser(parser.NewJSON()).
		Filter(ff).
		Matcher(m).
		Output(out).
		AlertSink(&stderr).
		Config(Config{Workers: 2, BufferSize: 16}).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return p, &stdout, &stderr
}

func TestPipeline_FiltersJSONField(t *testing.T) {
	input := strings.Join([]string{
		`{"level":"info","msg":"hello"}`,
		`{"level":"error","msg":"boom"}`,
		`{"level":"info","msg":"world"}`,
	}, "\n") + "\n"

	f, _ := filter.FromSpec("level=error")
	p, stdout, _ := buildPipeline(t, input, f, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, `"level":"error"`) {
		t.Errorf("expected error line in output, got %q", got)
	}
	if strings.Contains(got, `"level":"info"`) {
		t.Errorf("info line should have been filtered, got %q", got)
	}
}

func TestPipeline_AlertEmittedToStderr(t *testing.T) {
	var lines []string
	for i := 0; i < 5; i++ {
		lines = append(lines, `{"msg":"OOM killed worker"}`)
	}
	input := strings.Join(lines, "\n") + "\n"

	m, _ := matcher.New([]matcher.Spec{{
		Pattern: "OOM", Threshold: 3, Window: 30 * time.Second,
	}})
	p, _, stderr := buildPipeline(t, input, filter.Chain{}, m)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(stderr.String(), "[ALERT]") {
		t.Errorf("expected alert on stderr, got %q", stderr.String())
	}
}
