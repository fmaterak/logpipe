package source

import (
	"bufio"
	"context"
	"io"
)

// StdinSource reads lines from any io.Reader (stdin in production, an
// in-memory reader in tests). It is the simpler of the two sources: read until
// EOF, then close the output channel.
type StdinSource struct {
	r   io.Reader
	ctx context.Context
}

func NewStdin(ctx context.Context, r io.Reader) *StdinSource {
	return &StdinSource{r: r, ctx: ctx}
}

func (StdinSource) Name() string { return "stdin" }

func (s *StdinSource) Run(out chan<- string) error {
	defer close(out)
	scanner := bufio.NewScanner(s.r)
	// Default 64 KiB buffer is too small for some structured logs. Bump to
	// 1 MiB which comfortably handles deeply nested JSON without becoming
	// unbounded.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case out <- scanner.Text():
		}
	}
	return scanner.Err()
}
