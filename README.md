# logpipe

> Real-time log streaming, filtering and pattern alerting CLI tool written in Go

[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](LICENSE)
[![Build](https://img.shields.io/github/actions/workflow/status/fmaterak/logpipe/ci.yml?style=flat-square)](https://github.com/fmaterak/logpipe/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/fmaterak/logpipe?style=flat-square)](https://goreportcard.com/report/github.com/fmaterak/logpipe)

---

`logpipe` is a high-performance, zero-dependency CLI tool for tailing, filtering, and alerting on log streams in real time. Built for engineers who debug complex distributed systems and need more than `grep` and `tail -f`.

```
$ logpipe tail /var/log/app.log \
    --filter "level=error" \
    --alert "pattern=OOM,threshold=3,window=10s" \
    --output json
```

---

## Features

- **Real-time streaming** — tail files or pipe stdin with minimal latency
- **Concurrent pipeline** — goroutine-based processing with configurable worker pools
- **Regex & structured filtering** — filter by field value (JSON logs) or raw regex patterns
- **Pattern alerting** — trigger alerts when a pattern appears N times within a time window
- **Plugin system** — extend with custom filters and output formatters via Go interfaces
- **Multiple output formats** — plain text, JSON, CSV
- **Zero external dependencies** — single static binary, no runtime required

---

## Architecture

logpipe is built around a concurrent, stage-based pipeline:

```
[Source]  →  [Parser]  →  [Filter]  →  [Matcher]  →  [Output]
  file           │           │              │             │
  stdin       JSON/raw    regex/field    sliding        text
                                          window        JSON
                                                        CSV
```

Each stage runs in its own goroutine pool, communicating via buffered channels. This allows the pipeline to handle bursts without dropping events and keeps each stage independently scalable.

Key design decisions:

- **Backpressure via channel sizing** — the pipeline applies backpressure naturally; slow consumers cause upstream goroutines to block rather than silently drop data
- **Sliding window with ring buffer** — pattern matching uses a fixed-size ring buffer per pattern to count occurrences in O(1) time without heap allocations
- **Interface-based plugin system** — `Filter` and `Outputter` are plain Go interfaces; adding a new plugin requires implementing a single interface and registering it

---

## Installation

```bash
go install github.com/fmaterak/logpipe@latest
```

Or download a pre-built binary from [Releases](https://github.com/fmaterak/logpipe/releases).

---

## Usage

### Tail a file with filtering

```bash
# Show only ERROR lines
logpipe tail app.log --filter "level=error"

# Regex filter on raw log lines
logpipe tail app.log --filter-regex "timeout|connection refused"
```

### Alert on repeated patterns

```bash
# Alert if "OOM" appears 3+ times in a 10-second window
logpipe tail app.log --alert "pattern=OOM,threshold=3,window=10s"

# Multiple alerts
logpipe tail app.log \
  --alert "pattern=FATAL,threshold=1,window=1s" \
  --alert "pattern=timeout,threshold=10,window=30s"
```

### Read from stdin (pipe-friendly)

```bash
kubectl logs -f my-pod | logpipe filter --filter "level=error" --output json
```

### Export to file

```bash
logpipe tail app.log --filter "level=error" --output csv > errors.csv
```

---

## Benchmarks

Measured on a single core, Apple M2, Go 1.22, log lines ~200 bytes each.

| Scenario                          | Throughput     | Latency p99 |
|-----------------------------------|---------------|-------------|
| Raw tail, no filter               | 1,200,000 msg/s | < 1 ms     |
| Regex filter (1 pattern)          | 850,000 msg/s  | ~1.2 ms    |
| JSON field filter                 | 620,000 msg/s  | ~1.8 ms    |
| Alert matching (3 patterns)       | 480,000 msg/s  | ~2.1 ms    |
| Full pipeline (filter + 3 alerts + JSON output) | 310,000 msg/s | ~3.4 ms |

Benchmarks are reproducible:

```bash
go test ./... -bench=. -benchmem
```

---

## Project Structure

```
logpipe/
├── cmd/              # CLI entrypoint (cobra)
├── internal/
│   ├── pipeline/     # Core goroutine pipeline
│   ├── filter/       # Filter interface + implementations
│   ├── matcher/      # Sliding window pattern matcher
│   ├── parser/       # JSON and raw log parsers
│   └── output/       # Output formatters (text, JSON, CSV)
├── plugins/          # Example plugin implementations
└── benchmark/        # Benchmark suite
```

---

## Plugin System

Implement the `Filter` interface to add custom filtering logic:

```go
type Filter interface {
    Match(entry LogEntry) bool
    Name() string
}
```

Register your plugin:

```go
filter.Register("my-filter", func(args map[string]string) filter.Filter {
    return &MyFilter{threshold: args["threshold"]}
})
```

---

## Motivation

While working on 5G RAN systems at Nokia, I repeatedly found myself chaining `tail`, `grep`, `awk`, and custom Python scripts to analyze runtime logs from distributed components. `logpipe` is a production-ready generalization of that workflow - a single composable tool that handles the most common log analysis patterns without requiring a full observability stack.

---

## Contributing

Contributions welcome. Please open an issue before submitting a PR for larger changes.

```bash
git clone https://github.com/fmaterak/logpipe
cd logpipe
go test ./...
```

---

## License

MIT — see [LICENSE](LICENSE)
