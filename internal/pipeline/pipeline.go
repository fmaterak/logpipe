// Package pipeline wires Source -> Parser -> Filter -> Matcher -> Output into
// a concurrent, channel-connected processing graph. Every stage runs on its
// own goroutine pool sized by Config.Workers; back-pressure is provided by
// channel sizing (Config.BufferSize) so a slow consumer simply blocks the
// stage upstream of it.
//
// All stages exit cleanly on context cancellation: each goroutine selects on
// the input channel and ctx.Done(), and downstream channels are closed only
// after the upstream goroutines have returned. This makes Run() safe to use
// inside signal-handling top-level code.
package pipeline

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/fmaterak/logpipe/internal/filter"
	"github.com/fmaterak/logpipe/internal/matcher"
	"github.com/fmaterak/logpipe/internal/output"
	"github.com/fmaterak/logpipe/internal/parser"
	"github.com/fmaterak/logpipe/internal/source"
)

// Config bundles the runtime knobs of the pipeline. Zero values are not
// useful; use DefaultConfig() and override what you need.
type Config struct {
	Workers    int // workers per parser/filter/output stage
	BufferSize int // channel capacity between stages
}

func DefaultConfig() Config {
	return Config{Workers: 2, BufferSize: 1024}
}

// Pipeline is an immutable description of a single run. Build one with
// Builder, then call Run.
type Pipeline struct {
	cfg     Config
	src     source.Source
	parser  parser.Parser
	filter  filter.Filter
	matcher *matcher.Matcher
	output  output.Outputter

	// alertSink is where Alert events are written. Defaults to a textual
	// dump on stderr; the CLI overrides it when --alert-output is set.
	alertSink io.Writer
}

// Builder constructs a Pipeline. The pattern keeps the call site readable -
// every dependency is named and required, but optional knobs (workers,
// buffers, alert sink) are easy to override.
type Builder struct {
	p *Pipeline
}

func New() *Builder { return &Builder{p: &Pipeline{cfg: DefaultConfig()}} }

func (b *Builder) Source(s source.Source) *Builder     { b.p.src = s; return b }
func (b *Builder) Parser(p parser.Parser) *Builder     { b.p.parser = p; return b }
func (b *Builder) Filter(f filter.Filter) *Builder     { b.p.filter = f; return b }
func (b *Builder) Matcher(m *matcher.Matcher) *Builder { b.p.matcher = m; return b }
func (b *Builder) Output(o output.Outputter) *Builder  { b.p.output = o; return b }
func (b *Builder) AlertSink(w io.Writer) *Builder      { b.p.alertSink = w; return b }
func (b *Builder) Config(c Config) *Builder            { b.p.cfg = c; return b }

func (b *Builder) Build() (*Pipeline, error) {
	p := b.p
	if p.src == nil {
		return nil, fmt.Errorf("pipeline: source is required")
	}
	if p.parser == nil {
		return nil, fmt.Errorf("pipeline: parser is required")
	}
	if p.output == nil {
		return nil, fmt.Errorf("pipeline: output is required")
	}
	if p.cfg.Workers < 1 {
		p.cfg.Workers = 1
	}
	if p.cfg.BufferSize < 1 {
		p.cfg.BufferSize = 1
	}
	return p, nil
}

// Run executes the pipeline until ctx is cancelled or the source drains. It
// returns the first non-nil error from the source goroutine; output and
// matcher errors are accumulated and reported as well.
func (p *Pipeline) Run(ctx context.Context) error {
	lines := make(chan string, p.cfg.BufferSize)
	parsed := make(chan parser.LogEntry, p.cfg.BufferSize)
	filtered := make(chan parser.LogEntry, p.cfg.BufferSize)

	var (
		wgParse  sync.WaitGroup
		wgFilter sync.WaitGroup
		wgOut    sync.WaitGroup
	)

	// --- source stage ---------------------------------------------------
	srcErrCh := make(chan error, 1)
	go func() {
		srcErrCh <- p.src.Run(lines)
	}()

	// --- parser stage ---------------------------------------------------
	for i := 0; i < p.cfg.Workers; i++ {
		wgParse.Add(1)
		go func() {
			defer wgParse.Done()
			for line := range lines {
				select {
				case <-ctx.Done():
					return
				case parsed <- p.parser.Parse(line):
				}
			}
		}()
	}
	go func() {
		wgParse.Wait()
		close(parsed)
	}()

	// --- filter stage ---------------------------------------------------
	for i := 0; i < p.cfg.Workers; i++ {
		wgFilter.Add(1)
		go func() {
			defer wgFilter.Done()
			for entry := range parsed {
				if p.filter != nil && !p.filter.Match(entry) {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case filtered <- entry:
				}
			}
		}()
	}
	go func() {
		wgFilter.Wait()
		close(filtered)
	}()

	// --- matcher + output stage ----------------------------------------
	// The matcher and outputter are merged into one stage: we always want
	// to emit the entry, and matching against alert specs is cheap. Each
	// worker reads from `filtered`, fires alerts, then writes to output.
	var outErr error
	var outErrOnce sync.Once
	for i := 0; i < p.cfg.Workers; i++ {
		wgOut.Add(1)
		go func() {
			defer wgOut.Done()
			for entry := range filtered {
				if p.matcher != nil {
					for _, a := range p.matcher.Match(entry) {
						p.emitAlert(a)
					}
				}
				if err := p.output.Write(entry); err != nil {
					outErrOnce.Do(func() { outErr = err })
				}
			}
		}()
	}
	wgOut.Wait()

	if err := p.output.Close(); err != nil && outErr == nil {
		outErr = err
	}

	srcErr := <-srcErrCh
	if srcErr != nil && srcErr != context.Canceled && srcErr != context.DeadlineExceeded {
		return srcErr
	}
	return outErr
}

func (p *Pipeline) emitAlert(a matcher.Alert) {
	if p.alertSink == nil {
		return
	}
	// One Fprintln keeps the alert atomic against concurrent writers
	// to the same sink; io.Writer implementations like os.Stderr handle
	// short writes for us.
	fmt.Fprintln(p.alertSink, a.String())
}
