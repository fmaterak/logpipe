package output

import (
	"bufio"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/fmaterak/logpipe/internal/parser"
)

// jsonOutputter emits one JSON object per line (NDJSON / JSON Lines). Each
// object always carries a `_raw` field with the original input and a
// `_timestamp` field; parsed fields, if any, are merged at the top level.
type jsonOutputter struct {
	mu  sync.Mutex
	bw  *bufio.Writer
	enc *json.Encoder
}

func (o *jsonOutputter) Name() string { return "json" }

func (o *jsonOutputter) Write(entry parser.LogEntry) error {
	obj := make(map[string]any, len(entry.Fields)+2)
	obj["_raw"] = entry.Raw
	obj["_timestamp"] = entry.Timestamp.Format(time.RFC3339Nano)
	for k, v := range entry.Fields {
		obj[k] = v
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	// json.Encoder.Encode appends a newline, which gives us NDJSON for free.
	if err := o.enc.Encode(obj); err != nil {
		return err
	}
	return o.bw.Flush()
}

func (o *jsonOutputter) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.bw.Flush()
}

func init() {
	Register("json", func(w io.Writer) Outputter {
		bw := bufio.NewWriter(w)
		enc := json.NewEncoder(bw)
		// Avoid escaping HTML special chars in log payloads: it is rarely
		// what users want and makes diffing log output noisy.
		enc.SetEscapeHTML(false)
		return &jsonOutputter{bw: bw, enc: enc}
	})
}
