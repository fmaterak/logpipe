// Package plugins is the home for example filters that demonstrate the
// plugin contract documented in the README.
//
// To enable a plugin, blank-import this package from your main package - the
// init() function below will register the filter with the global registry,
// after which it can be referenced from the CLI as `--filter "threshold:..."`.
package plugins

import (
	"fmt"
	"strconv"

	"github.com/fmaterak/logpipe/internal/filter"
	"github.com/fmaterak/logpipe/internal/parser"
)

// ThresholdFilter passes entries whose numeric field crosses a threshold in a
// configurable direction. CLI form:
//
//	--filter "threshold:field=latency_ms,op=gt,value=100"
//
// Operators: lt, lte, gt, gte, eq.
//
// The filter only fires for entries that have parsed numeric fields; raw
// lines are rejected. This keeps the semantics predictable and avoids
// "looks-like-a-number" surprises in unstructured text.
type ThresholdFilter struct {
	Field string
	Op    string
	Value float64
}

func (t *ThresholdFilter) Name() string { return "threshold" }

func (t *ThresholdFilter) Match(entry parser.LogEntry) bool {
	if entry.Fields == nil {
		return false
	}
	raw, ok := entry.Fields[t.Field]
	if !ok {
		return false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return false
	}
	switch t.Op {
	case "lt":
		return v < t.Value
	case "lte":
		return v <= t.Value
	case "gt":
		return v > t.Value
	case "gte":
		return v >= t.Value
	case "eq":
		return v == t.Value
	}
	return false
}

func init() {
	filter.Register("threshold", func(args map[string]string) (filter.Filter, error) {
		field, ok := args["field"]
		if !ok || field == "" {
			return nil, fmt.Errorf("threshold filter requires 'field'")
		}
		op := args["op"]
		switch op {
		case "lt", "lte", "gt", "gte", "eq":
		case "":
			return nil, fmt.Errorf("threshold filter requires 'op' (one of lt,lte,gt,gte,eq)")
		default:
			return nil, fmt.Errorf("threshold filter: unknown op %q", op)
		}
		valStr, ok := args["value"]
		if !ok {
			return nil, fmt.Errorf("threshold filter requires 'value'")
		}
		v, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return nil, fmt.Errorf("threshold filter: value %q is not numeric: %w", valStr, err)
		}
		return &ThresholdFilter{Field: field, Op: op, Value: v}, nil
	})
}
