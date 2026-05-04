// Package benchmark hosts representative end-to-end benchmarks. They are
// intentionally separated from the package-level micro-benchmarks because
// readers of the README expect throughput numbers from a realistic pipeline,
// not isolated function timings.
//
// Run with: go test ./benchmark -bench=. -benchmem
package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fmaterak/logpipe/internal/filter"
	"github.com/fmaterak/logpipe/internal/matcher"
	"github.com/fmaterak/logpipe/internal/output"
	"github.com/fmaterak/logpipe/internal/parser"
	"github.com/fmaterak/logpipe/internal/pipeline"
	"github.com/fmaterak/logpipe/internal/source"
)

// generateLines returns n synthetic log lines roughly 200 bytes each, mixing
// info / error / OOM messages so filters and alerts have realistic work to do.
func generateLines(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		switch i % 5 {
		case 0:
			fmt.Fprintf(&b, `{"level":"info","i":%d,"msg":"request handled latency_ms=%d","host":"node-01","trace":"abc123","span":"00001"}`+"\n", i, i%200)
		case 1:
			fmt.Fprintf(&b, `{"level":"warn","i":%d,"msg":"slow query latency_ms=%d","host":"node-02","trace":"abc124","span":"00002"}`+"\n", i, 200+i%200)
		case 2:
			fmt.Fprintf(&b, `{"level":"error","i":%d,"msg":"timeout calling backend","host":"node-03","trace":"abc125","span":"00003"}`+"\n", i)
		case 3:
			fmt.Fprintf(&b, `{"level":"error","i":%d,"msg":"OOM killed worker","host":"node-04","trace":"abc126","span":"00004"}`+"\n", i)
		default:
			fmt.Fprintf(&b, `{"level":"info","i":%d,"msg":"heartbeat","host":"node-05","trace":"abc127","span":"00005"}`+"\n", i)
		}
	}
	return b.String()
}

func runPipeline(b *testing.B, ff filter.Filter, m *matcher.Matcher, outName string, lines string) {
	b.Helper()
	for i := 0; i < b.N; i++ {
		out, err := output.Get(outName, &bytes.Buffer{})
		if err != nil {
			b.Fatalf("output: %v", err)
		}
		src := source.NewStdin(context.Background(), strings.NewReader(lines))
		p, err := pipeline.New().
			Source(src).
			Parser(parser.NewJSON()).
			Filter(ff).
			Matcher(m).
			Output(out).
			AlertSink(&bytes.Buffer{}).
			Config(pipeline.Config{Workers: 2, BufferSize: 1024}).
			Build()
		if err != nil {
			b.Fatalf("build: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := p.Run(ctx); err != nil {
			cancel()
			b.Fatalf("run: %v", err)
		}
		cancel()
	}
}

const benchN = 10_000

func BenchmarkPipeline_Raw(b *testing.B) {
	lines := generateLines(benchN)
	b.SetBytes(int64(len(lines)))
	b.ResetTimer()
	runPipeline(b, filter.Chain{}, nil, "text", lines)
}

func BenchmarkPipeline_RegexFilter(b *testing.B) {
	lines := generateLines(benchN)
	f, _ := filter.FromSpec("regex:pattern=timeout|connection refused")
	b.SetBytes(int64(len(lines)))
	b.ResetTimer()
	runPipeline(b, f, nil, "text", lines)
}

func BenchmarkPipeline_FieldFilter(b *testing.B) {
	lines := generateLines(benchN)
	f, _ := filter.FromSpec("level=error")
	b.SetBytes(int64(len(lines)))
	b.ResetTimer()
	runPipeline(b, f, nil, "text", lines)
}

func BenchmarkPipeline_AlertMatching(b *testing.B) {
	lines := generateLines(benchN)
	specs := []matcher.Spec{
		{Pattern: "OOM", Threshold: 3, Window: 10 * time.Second},
		{Pattern: "timeout", Threshold: 5, Window: 30 * time.Second},
		{Pattern: "FATAL", Threshold: 1, Window: 1 * time.Second},
	}
	m, err := matcher.New(specs)
	if err != nil {
		b.Fatalf("new matcher: %v", err)
	}
	b.SetBytes(int64(len(lines)))
	b.ResetTimer()
	runPipeline(b, filter.Chain{}, m, "text", lines)
}

func BenchmarkPipeline_Full(b *testing.B) {
	lines := generateLines(benchN)
	f, _ := filter.FromSpec("level=error")
	specs := []matcher.Spec{
		{Pattern: "OOM", Threshold: 3, Window: 10 * time.Second},
		{Pattern: "timeout", Threshold: 5, Window: 30 * time.Second},
		{Pattern: "FATAL", Threshold: 1, Window: 1 * time.Second},
	}
	m, err := matcher.New(specs)
	if err != nil {
		b.Fatalf("new matcher: %v", err)
	}
	b.SetBytes(int64(len(lines)))
	b.ResetTimer()
	runPipeline(b, f, m, "json", lines)
}
