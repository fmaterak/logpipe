package filter

import (
	"fmt"
	"strings"

	"github.com/fmaterak/logpipe/internal/parser"
)

// FieldFilter matches a JSON field against a value with case-insensitive
// equality. It is the workhorse for structured logs - "level=error" reads
// naturally and is what most users want.
//
// When the entry has no parsed fields (RawParser was used) the filter falls
// back to a substring search of "<key>=<value>" against the raw line. This
// is best-effort but lets a single filter spec work across both modes.
type FieldFilter struct {
	Key   string
	Value string
}

func (f *FieldFilter) Name() string { return "field" }

func (f *FieldFilter) Match(entry parser.LogEntry) bool {
	if entry.Fields != nil {
		v, ok := entry.Fields[f.Key]
		if !ok {
			return false
		}
		return strings.EqualFold(v, f.Value)
	}
	needle := f.Key + "=" + f.Value
	return strings.Contains(strings.ToLower(entry.Raw), strings.ToLower(needle))
}

func init() {
	Register("field", func(args map[string]string) (Filter, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("field filter expects exactly one key=value pair, got %d", len(args))
		}
		for k, v := range args {
			return &FieldFilter{Key: k, Value: v}, nil
		}
		return nil, fmt.Errorf("unreachable")
	})
}
