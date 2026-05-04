// Package parser converts raw log lines into LogEntry values that the rest of
// the pipeline can reason about. Two parsers are provided: a no-op Raw parser
// and a JSON parser that flattens top-level object fields into a string map.
package parser

import "time"

// LogEntry is the canonical record flowing through the pipeline. The Raw field
// always holds the original line; Fields is populated by structured parsers
// (e.g. JSON). Timestamp is set to the moment the line entered the pipeline,
// not parsed from the line itself, to keep parsing cheap.
type LogEntry struct {
	Raw       string
	Fields    map[string]string
	Timestamp time.Time
}

// Parser turns a single line of input into a LogEntry. Implementations must be
// safe to call concurrently from multiple goroutines.
type Parser interface {
	Parse(line string) LogEntry
	Name() string
}
