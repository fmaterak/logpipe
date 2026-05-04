package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fmaterak/logpipe/internal/filter"
	"github.com/fmaterak/logpipe/internal/matcher"
	"github.com/fmaterak/logpipe/internal/output"
	"github.com/fmaterak/logpipe/internal/parser"
	"github.com/fmaterak/logpipe/internal/pipeline"
	"github.com/fmaterak/logpipe/internal/source"
)

// boolFlags lists the names of boolean flags. They take no separate value
// argument, so reorderFlagsFirst must not greedily consume the next token.
// Keep this in sync with registerFlags above.
var boolFlags = map[string]struct{}{
	"from-start": {},
}

// reorderFlagsFirst moves flag tokens (and their values) to the front of args
// so positionals can appear anywhere on the command line. We treat any token
// starting with '-' as a flag; for non-boolean flags written as "-name value"
// (no '='), the following token is also reclassified as a flag value.
func reorderFlagsFirst(args []string) []string {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if eq := strings.Index(name, "="); eq >= 0 {
				name = name[:eq]
				continue
			}
			if _, isBool := boolFlags[name]; isBool {
				continue
			}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		positionals = append(positionals, a)
	}
	return append(flags, positionals...)
}

// stringSlice lets the same flag be supplied multiple times. The README shows
// the user passing --filter and --alert more than once, so this is the natural
// shape for those flags.
type stringSlice []string

func (s *stringSlice) String() string     { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error { *s = append(*s, v); return nil }

// runOptions is shared by `tail` and `filter` so both subcommands accept the
// same flag surface. Only the source differs.
type runOptions struct {
	filters      stringSlice
	regexFilters stringSlice
	alerts       stringSlice
	parserName   string
	outputName   string
	workers      int
	bufferSize   int
	fromStart    bool
}

func registerFlags(fs *flag.FlagSet, o *runOptions) {
	fs.Var(&o.filters, "filter", "field filter spec, e.g. \"level=error\" or \"name:k=v\" (repeatable)")
	fs.Var(&o.regexFilters, "filter-regex", "regex applied to the raw line (repeatable)")
	fs.Var(&o.alerts, "alert", "alert spec \"pattern=...,threshold=N,window=10s\" (repeatable)")
	fs.StringVar(&o.parserName, "parser", "json", "parser to use: json or raw (json falls back to raw on non-JSON lines)")
	fs.StringVar(&o.outputName, "output", "text", "output format: text, json, csv")
	fs.IntVar(&o.workers, "workers", 2, "worker goroutines per pipeline stage")
	fs.IntVar(&o.bufferSize, "buffer", 1024, "channel buffer size between stages")
	fs.BoolVar(&o.fromStart, "from-start", true, "for tail: read existing file content before tailing")
}

func runTail(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tail", flag.ContinueOnError)
	var opts runOptions
	registerFlags(fs, &opts)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: logpipe tail [flags] [path]\n\n")
		fs.PrintDefaults()
	}
	// flag.Parse stops at the first non-flag, which makes the README's
	// `logpipe tail /path --filter ...` ordering inconvenient. Reorder so
	// flag-like tokens come first, mimicking what cobra/getopt-long do.
	if err := fs.Parse(reorderFlagsFirst(args)); err != nil {
		return err
	}

	rest := fs.Args()
	var src source.Source
	switch len(rest) {
	case 0:
		src = source.NewStdin(ctx, os.Stdin)
	case 1:
		src = source.NewFile(ctx, rest[0], opts.fromStart)
	default:
		return fmt.Errorf("tail accepts at most one path, got %d", len(rest))
	}

	return runWith(ctx, src, &opts, os.Stdout, os.Stderr)
}

func runFilter(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("filter", flag.ContinueOnError)
	var opts runOptions
	registerFlags(fs, &opts)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: logpipe filter [flags]   (reads stdin)\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("filter does not accept positional arguments; pipe stdin instead")
	}
	return runWith(ctx, source.NewStdin(ctx, os.Stdin), &opts, os.Stdout, os.Stderr)
}

// runWith builds and runs the pipeline. It is split out from the subcommand
// handlers so they only have to deal with arg parsing.
func runWith(ctx context.Context, src source.Source, opts *runOptions, stdout, stderr io.Writer) error {
	parsr, err := buildParser(opts.parserName)
	if err != nil {
		return err
	}

	chain, err := buildFilters(opts.filters, opts.regexFilters)
	if err != nil {
		return err
	}

	specs := make([]matcher.Spec, 0, len(opts.alerts))
	for _, a := range opts.alerts {
		spec, err := matcher.ParseSpec(a)
		if err != nil {
			return fmt.Errorf("alert %q: %w", a, err)
		}
		specs = append(specs, spec)
	}
	m, err := matcher.New(specs)
	if err != nil {
		return err
	}

	out, err := output.Get(opts.outputName, stdout)
	if err != nil {
		return err
	}

	p, err := pipeline.New().
		Source(src).
		Parser(parsr).
		Filter(chain).
		Matcher(m).
		Output(out).
		AlertSink(stderr).
		Config(pipeline.Config{Workers: opts.workers, BufferSize: opts.bufferSize}).
		Build()
	if err != nil {
		return err
	}

	return p.Run(ctx)
}

func buildParser(name string) (parser.Parser, error) {
	switch name {
	case "json":
		return parser.NewJSON(), nil
	case "raw":
		return parser.NewRaw(), nil
	default:
		return nil, fmt.Errorf("unknown parser %q (use json or raw)", name)
	}
}

func buildFilters(fields, regexes []string) (filter.Filter, error) {
	chain := filter.Chain{}
	for _, spec := range fields {
		f, err := filter.FromSpec(spec)
		if err != nil {
			return nil, err
		}
		chain.Filters = append(chain.Filters, f)
	}
	for _, pattern := range regexes {
		f, err := filter.FromSpec("regex:pattern=" + pattern)
		if err != nil {
			return nil, err
		}
		chain.Filters = append(chain.Filters, f)
	}
	return chain, nil
}
