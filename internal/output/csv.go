package output

import (
	"bufio"
	"encoding/csv"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/fmaterak/logpipe/internal/parser"
)

// csvOutputter emits one CSV row per entry. The first row is a header.
//
// CSV columns are derived dynamically: the schema is "_timestamp,_raw,
// <fields...>" where <fields...> is the union of every key seen so far,
// sorted alphabetically. When a new field appears we re-emit the header.
// This is a pragmatic choice - real CSV consumers expect a fixed schema, and
// re-emitting the header makes the change explicit instead of silently
// shifting columns.
type csvOutputter struct {
	mu            sync.Mutex
	bw            *bufio.Writer
	w             *csv.Writer
	keys          []string
	keySeen       map[string]struct{}
	headerEmitted bool
}

func (o *csvOutputter) Name() string { return "csv" }

func (o *csvOutputter) Write(entry parser.LogEntry) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	added := false
	for k := range entry.Fields {
		if _, ok := o.keySeen[k]; !ok {
			o.keySeen[k] = struct{}{}
			o.keys = append(o.keys, k)
			added = true
		}
	}
	if added {
		sort.Strings(o.keys)
	}
	if !o.headerEmitted || added {
		if err := o.writeHeaderLocked(); err != nil {
			return err
		}
		o.headerEmitted = true
	}

	row := make([]string, 0, 2+len(o.keys))
	row = append(row, entry.Timestamp.Format(time.RFC3339Nano), entry.Raw)
	for _, k := range o.keys {
		row = append(row, entry.Fields[k])
	}
	if err := o.w.Write(row); err != nil {
		return err
	}
	o.w.Flush()
	if err := o.w.Error(); err != nil {
		return err
	}
	return o.bw.Flush()
}

func (o *csvOutputter) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.w.Flush()
	return o.bw.Flush()
}

func (o *csvOutputter) writeHeaderLocked() error {
	header := append([]string{"_timestamp", "_raw"}, o.keys...)
	if err := o.w.Write(header); err != nil {
		return err
	}
	o.w.Flush()
	return o.w.Error()
}

func init() {
	Register("csv", func(w io.Writer) Outputter {
		bw := bufio.NewWriter(w)
		return &csvOutputter{
			bw:      bw,
			w:       csv.NewWriter(bw),
			keySeen: map[string]struct{}{},
		}
	})
}
