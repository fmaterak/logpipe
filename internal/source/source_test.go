package source

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func collect(ch <-chan string, until time.Duration) []string {
	deadline := time.After(until)
	var out []string
	for {
		select {
		case s, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, s)
		case <-deadline:
			return out
		}
	}
}

func TestStdinSource_ReadsAllLines(t *testing.T) {
	r := strings.NewReader("a\nb\nc\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan string, 8)
	src := NewStdin(ctx, r)
	if err := src.Run(out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := []string{}
	for s := range out {
		got = append(got, s)
	}
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestFileSource_TailsAppendedContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.txt")
	if err := os.WriteFile(path, []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan string, 8)
	src := NewFile(ctx, path, true)
	src.PollInterval = 20 * time.Millisecond

	doneRun := make(chan error, 1)
	go func() { doneRun <- src.Run(out) }()

	// First line should arrive promptly.
	if line := <-out; line != "first" {
		t.Fatalf("first line: got %q", line)
	}

	// Append after a short delay; the source must pick it up.
	go func() {
		time.Sleep(50 * time.Millisecond)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Errorf("open append: %v", err)
			return
		}
		f.WriteString("second\n")
		f.Close()
	}()

	select {
	case line := <-out:
		if line != "second" {
			t.Errorf("appended line: got %q", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for appended line")
	}

	cancel()
	<-doneRun
}
