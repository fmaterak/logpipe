// Package output provides Outputter implementations for the formats listed in
// the README: plain text, JSON and CSV. Outputters are constructed via Get()
// from the registry to keep the CLI plumbing format-agnostic.
package output

import (
	"fmt"
	"io"
	"sync"

	"github.com/fmaterak/logpipe/internal/parser"
)

// Outputter writes a LogEntry to its underlying sink. Implementations must be
// safe to call concurrently because the pipeline may run multiple output
// workers in parallel - they synchronise writes internally.
type Outputter interface {
	Write(entry parser.LogEntry) error
	Close() error
	Name() string
}

// Factory builds an Outputter that writes to w.
type Factory func(w io.Writer) Outputter

var (
	regMu    sync.RWMutex
	registry = map[string]Factory{}
)

func Register(name string, f Factory) {
	regMu.Lock()
	defer regMu.Unlock()
	registry[name] = f
}

func Names() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	return names
}

func Get(name string, w io.Writer) (Outputter, error) {
	regMu.RLock()
	f, ok := registry[name]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown output format %q (registered: %v)", name, Names())
	}
	return f(w), nil
}
