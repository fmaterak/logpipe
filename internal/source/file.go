package source

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"time"
)

// FileSource implements `tail -f` semantics with two modes:
//
//   - FromStart=true: read the existing content first, then keep watching for
//     appends. This is the default "tail with history" behaviour.
//   - FromStart=false: skip to EOF and only emit lines appended after start.
//
// The implementation deliberately uses polling rather than fsnotify/inotify
// because the README highlights "zero external dependencies". A short poll
// interval (default 100ms) keeps the perceived latency well under the
// "minimal latency" claim while costing one stat() per interval - cheap on
// any modern OS.
//
// Truncation is detected by comparing file size to the last read offset; on
// truncate the source seeks back to 0 and continues. This matches what users
// expect from `tail -F` and avoids dropping the entire file on log rotation
// performed by simple truncation.
type FileSource struct {
	Path         string
	FromStart    bool
	PollInterval time.Duration

	ctx context.Context
}

func NewFile(ctx context.Context, path string, fromStart bool) *FileSource {
	return &FileSource{
		Path:         path,
		FromStart:    fromStart,
		PollInterval: 100 * time.Millisecond,
		ctx:          ctx,
	}
}

func (FileSource) Name() string { return "file" }

func (s *FileSource) Run(out chan<- string) error {
	defer close(out)

	f, err := os.Open(s.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	if !s.FromStart {
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			return err
		}
	}

	reader := bufio.NewReader(f)
	// Carry-over for partial lines that arrive before the trailing '\n'.
	var pending []byte

	ticker := time.NewTicker(s.PollInterval)
	defer ticker.Stop()

	for {
		// Drain whatever bytes are available right now.
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				pending = append(pending, line...)
				if line[len(line)-1] == '\n' {
					trimmed := pending[:len(pending)-1]
					if len(trimmed) > 0 && trimmed[len(trimmed)-1] == '\r' {
						trimmed = trimmed[:len(trimmed)-1]
					}
					select {
					case <-s.ctx.Done():
						return s.ctx.Err()
					case out <- string(trimmed):
					}
					pending = pending[:0]
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return err
				}
				break
			}
		}

		// EOF: wait for new data or cancellation.
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-ticker.C:
		}

		// Detect truncation: if the file is now shorter than our position,
		// rewind and re-create the reader so we don't miss the new content.
		if cur, err := f.Seek(0, io.SeekCurrent); err == nil {
			if info, err := f.Stat(); err == nil && info.Size() < cur {
				if _, err := f.Seek(0, io.SeekStart); err != nil {
					return err
				}
				reader.Reset(f)
				pending = pending[:0]
			}
		}
	}
}
