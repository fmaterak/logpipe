// Package matcher implements pattern-based alerting on the log stream. Each
// alert tracks how many times its pattern has occurred within a sliding time
// window. When the count reaches the configured threshold the matcher emits
// an Alert event.
//
// The implementation uses a fixed-size ring buffer per alert to keep the hot
// path allocation-free: counting events in a window is O(1) amortised and
// requires only ring[head] = ts, head = (head+1) % cap.
package matcher

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fmaterak/logpipe/internal/parser"
)

// Alert is emitted when an alert specification is triggered. The Sample field
// holds the entry that pushed the count over the threshold so the user has
// concrete context when looking at the alert.
type Alert struct {
	Pattern   string
	Threshold int
	Window    time.Duration
	Count     int
	FiredAt   time.Time
	Sample    parser.LogEntry
}

func (a Alert) String() string {
	return fmt.Sprintf(
		"[ALERT] pattern=%q threshold=%d window=%s count=%d at=%s sample=%s",
		a.Pattern, a.Threshold, a.Window, a.Count,
		a.FiredAt.Format(time.RFC3339Nano), strings.TrimSpace(a.Sample.Raw),
	)
}

// Spec is the parsed form of a CLI --alert flag.
type Spec struct {
	Pattern   string
	Threshold int
	Window    time.Duration
}

// ParseSpec parses "pattern=...,threshold=N,window=10s" into a Spec. The
// pattern field is a regex and is allowed to contain commas because we only
// treat commas as separators between top-level k=v pairs - any comma inside
// the regex must be escaped with '\\,'.
func ParseSpec(s string) (Spec, error) {
	var spec Spec
	for _, part := range splitTopLevel(s, ',') {
		eq := strings.Index(part, "=")
		if eq < 0 {
			return spec, fmt.Errorf("alert argument %q is missing '='", part)
		}
		k := strings.TrimSpace(part[:eq])
		v := part[eq+1:]
		switch k {
		case "pattern":
			spec.Pattern = strings.ReplaceAll(v, `\,`, ",")
		case "threshold":
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil || n < 1 {
				return spec, fmt.Errorf("threshold must be a positive integer, got %q", v)
			}
			spec.Threshold = n
		case "window":
			d, err := time.ParseDuration(strings.TrimSpace(v))
			if err != nil || d <= 0 {
				return spec, fmt.Errorf("window must be a positive duration, got %q", v)
			}
			spec.Window = d
		default:
			return spec, fmt.Errorf("unknown alert key %q", k)
		}
	}
	if spec.Pattern == "" || spec.Threshold == 0 || spec.Window == 0 {
		return spec, fmt.Errorf("alert spec requires pattern, threshold and window")
	}
	return spec, nil
}

// splitTopLevel splits s on sep, treating "\\<sep>" as a literal sep. It is
// reused for parsing alert argument lists where the regex pattern may legally
// contain a comma.
func splitTopLevel(s string, sep byte) []string {
	out := make([]string, 0, 4)
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) && s[i+1] == sep {
			b.WriteByte(sep)
			i++
			continue
		}
		if c == sep {
			out = append(out, b.String())
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	out = append(out, b.String())
	return out
}

// alertState is the per-pattern runtime state. The ring buffer stores the
// timestamps of the most recent matches up to Threshold entries; once it is
// full, the oldest timestamp is overwritten. This is sufficient because we
// only need to know whether the Nth-most-recent match falls inside the window
// to decide if the threshold is crossed.
type alertState struct {
	spec Spec
	re   *regexp.Regexp

	mu   sync.Mutex
	ring []time.Time
	head int // index of the next slot to write
	size int // number of valid entries (0..len(ring))
	// lastFire tracks the most recent time this alert fired. We use it to
	// debounce repeated firings: an alert won't refire until the previous
	// "burst" has aged out of the window. Without this every subsequent
	// match would refire the alert as long as the window stayed full.
	lastFire time.Time
}

// Matcher evaluates a stream of LogEntry values against a set of alert
// specifications. Match is safe for concurrent use.
type Matcher struct {
	states []*alertState
}

func New(specs []Spec) (*Matcher, error) {
	m := &Matcher{}
	for _, s := range specs {
		re, err := regexp.Compile(s.Pattern)
		if err != nil {
			return nil, fmt.Errorf("alert pattern %q: %w", s.Pattern, err)
		}
		m.states = append(m.states, &alertState{
			spec: s,
			re:   re,
			ring: make([]time.Time, s.Threshold),
		})
	}
	return m, nil
}

// Match feeds one entry through every alert and returns any alerts that fired
// because of it. The slice is allocated only when alerts actually fire, so the
// happy path stays allocation-free.
func (m *Matcher) Match(entry parser.LogEntry) []Alert {
	if m == nil || len(m.states) == 0 {
		return nil
	}
	now := entry.Timestamp
	if now.IsZero() {
		now = time.Now()
	}

	var fired []Alert
	for _, st := range m.states {
		if !st.re.MatchString(entry.Raw) {
			continue
		}
		if a, ok := st.record(now, entry); ok {
			fired = append(fired, a)
		}
	}
	return fired
}

// record appends now to the ring buffer and returns an Alert if the window is
// satisfied. The function holds the per-alert mutex; contention is expected to
// be low because the ring is small (== threshold) and updates are O(1).
func (st *alertState) record(now time.Time, sample parser.LogEntry) (Alert, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.ring[st.head] = now
	st.head = (st.head + 1) % len(st.ring)
	if st.size < len(st.ring) {
		st.size++
	}

	if st.size < st.spec.Threshold {
		return Alert{}, false
	}

	// The oldest of the last `threshold` matches lives at the slot we are
	// about to overwrite (i.e. current head, since head moved forward).
	oldest := st.ring[st.head]
	if now.Sub(oldest) > st.spec.Window {
		return Alert{}, false
	}

	// Debounce: don't refire while the previous fire is still inside this
	// window. This keeps alert noise proportional to actual bursts.
	if !st.lastFire.IsZero() && now.Sub(st.lastFire) <= st.spec.Window {
		return Alert{}, false
	}
	st.lastFire = now

	return Alert{
		Pattern:   st.spec.Pattern,
		Threshold: st.spec.Threshold,
		Window:    st.spec.Window,
		Count:     st.size,
		FiredAt:   now,
		Sample:    sample,
	}, true
}
