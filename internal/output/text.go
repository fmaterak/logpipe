package output

import (
	"bufio"
	"io"
	"sync"

	"github.com/fmaterak/logpipe/internal/parser"
)

// textOutputter writes the raw line, preserving whatever the source produced.
// All output is funnelled through a buffered writer guarded by a mutex so
// concurrent worker goroutines do not interleave bytes mid-line.
type textOutputter struct {
	mu sync.Mutex
	bw *bufio.Writer
}

func (o *textOutputter) Name() string { return "text" }

func (o *textOutputter) Write(entry parser.LogEntry) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, err := o.bw.WriteString(entry.Raw); err != nil {
		return err
	}
	if len(entry.Raw) == 0 || entry.Raw[len(entry.Raw)-1] != '\n' {
		if err := o.bw.WriteByte('\n'); err != nil {
			return err
		}
	}
	return o.bw.Flush()
}

func (o *textOutputter) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.bw.Flush()
}

func init() {
	Register("text", func(w io.Writer) Outputter {
		return &textOutputter{bw: bufio.NewWriter(w)}
	})
}
