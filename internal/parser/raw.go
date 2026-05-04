package parser

import "time"

// RawParser is the trivial parser: it copies the input into LogEntry.Raw and
// leaves Fields nil. Used when logs are unstructured plain text.
type RawParser struct{}

func NewRaw() *RawParser { return &RawParser{} }

func (RawParser) Name() string { return "raw" }

func (RawParser) Parse(line string) LogEntry {
	return LogEntry{Raw: line, Timestamp: time.Now()}
}
