package parser

import (
	"encoding/json"
	"strconv"
	"time"
)

// JSONParser parses each line as a JSON object and flattens its top-level
// fields into LogEntry.Fields as strings. Non-string scalars are formatted via
// strconv; nested objects/arrays are stored as their JSON encoding.
//
// If a line is not valid JSON the parser falls back to RawParser semantics so
// the pipeline never drops data.
type JSONParser struct{}

func NewJSON() *JSONParser { return &JSONParser{} }

func (JSONParser) Name() string { return "json" }

func (JSONParser) Parse(line string) LogEntry {
	entry := LogEntry{Raw: line, Timestamp: time.Now()}

	// Fast reject: JSON objects always start with '{'. Avoid the cost of
	// invoking the decoder on lines that are obviously not JSON.
	if len(line) == 0 || line[0] != '{' {
		return entry
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return entry
	}

	fields := make(map[string]string, len(obj))
	for k, v := range obj {
		fields[k] = stringify(v)
	}
	entry.Fields = fields
	return entry
}

func stringify(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		// json.Unmarshal decodes all numbers as float64. Print integers
		// without a trailing .0 so field comparisons feel natural.
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return ""
		}
		return string(b)
	}
}
