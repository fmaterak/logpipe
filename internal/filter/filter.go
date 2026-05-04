// Package filter defines the Filter interface used by the pipeline and a
// small registry-based plugin system. Every concrete filter is registered
// under a name and constructed via FromSpec(), which keeps the CLI parsing
// logic decoupled from filter implementations.
package filter

import (
	"fmt"
	"strings"
	"sync"

	"github.com/fmaterak/logpipe/internal/parser"
)

// Filter decides whether a LogEntry should be forwarded to the next pipeline
// stage. Implementations MUST be safe for concurrent use because the filter
// stage may run multiple worker goroutines.
type Filter interface {
	Match(entry parser.LogEntry) bool
	Name() string
}

// Factory builds a Filter from a string-keyed argument map. The map mirrors
// the comma-separated key=value form accepted on the CLI.
type Factory func(args map[string]string) (Filter, error)

var (
	regMu    sync.RWMutex
	registry = map[string]Factory{}
)

// Register makes a filter available under name. Registration is intended to
// happen in init() functions of plugin packages; calling Register twice with
// the same name overwrites the previous factory, which is convenient for
// tests.
func Register(name string, f Factory) {
	regMu.Lock()
	defer regMu.Unlock()
	registry[name] = f
}

// Names returns the list of registered filter names; used by --help output.
func Names() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}

// FromSpec builds a filter from a CLI spec. The first form, "k=v", is shorthand
// for the "field" filter. The general form is "name:k1=v1,k2=v2" - a filter
// name followed by a colon and an argument list. This keeps simple cases short
// while making custom filters easy to wire from the command line.
func FromSpec(spec string) (Filter, error) {
	if spec == "" {
		return nil, fmt.Errorf("filter spec is empty")
	}

	name := "field"
	argStr := spec
	if i := strings.Index(spec, ":"); i >= 0 {
		name = strings.TrimSpace(spec[:i])
		argStr = spec[i+1:]
	}

	regMu.RLock()
	factory, ok := registry[name]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown filter %q (registered: %v)", name, Names())
	}

	args, err := parseArgs(argStr)
	if err != nil {
		return nil, fmt.Errorf("filter %q: %w", name, err)
	}
	return factory(args)
}

// parseArgs splits a "k1=v1,k2=v2" string into a map. Empty input yields an
// empty map. Whitespace around keys is trimmed but values are preserved
// verbatim so users can include leading/trailing spaces if they really want.
func parseArgs(s string) (map[string]string, error) {
	args := map[string]string{}
	if strings.TrimSpace(s) == "" {
		return args, nil
	}
	for _, part := range strings.Split(s, ",") {
		eq := strings.Index(part, "=")
		if eq < 0 {
			return nil, fmt.Errorf("argument %q is missing '='", part)
		}
		k := strings.TrimSpace(part[:eq])
		v := part[eq+1:]
		if k == "" {
			return nil, fmt.Errorf("empty argument key in %q", part)
		}
		args[k] = v
	}
	return args, nil
}

// Chain composes multiple filters with AND semantics: an entry must match
// every filter to pass. An empty chain accepts everything, which matches what
// users expect when no --filter flag is given.
type Chain struct {
	Filters []Filter
}

func (c Chain) Match(entry parser.LogEntry) bool {
	for _, f := range c.Filters {
		if !f.Match(entry) {
			return false
		}
	}
	return true
}

func (c Chain) Name() string { return "chain" }
