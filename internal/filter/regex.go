package filter

import (
	"fmt"
	"regexp"

	"github.com/fmaterak/logpipe/internal/parser"
)

// RegexFilter matches against the raw line of an entry. We deliberately do
// NOT match against parsed fields: the user's mental model when reaching for
// regex is "look at the line as text".
type RegexFilter struct {
	Re *regexp.Regexp
}

func (f *RegexFilter) Name() string { return "regex" }

func (f *RegexFilter) Match(entry parser.LogEntry) bool {
	return f.Re.MatchString(entry.Raw)
}

func init() {
	Register("regex", func(args map[string]string) (Filter, error) {
		pattern, ok := args["pattern"]
		if !ok {
			return nil, fmt.Errorf("regex filter requires 'pattern' argument")
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", pattern, err)
		}
		return &RegexFilter{Re: re}, nil
	})
}
