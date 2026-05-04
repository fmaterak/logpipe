// Package source provides line-oriented inputs for the pipeline. A Source
// pushes raw text lines into a channel until it is closed or its context is
// cancelled.
package source

// Source emits lines of input. Implementations close the returned channel
// when there is no more input and there will be no more in the future
// (e.g. stdin EOF). For long-running tail-like sources, the channel stays
// open until ctx is cancelled.
type Source interface {
	// Run drives the source and emits lines until ctx is cancelled or the
	// underlying input is exhausted. It must close `out` before returning.
	Run(out chan<- string) error
	Name() string
}
