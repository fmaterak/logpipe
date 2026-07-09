<div align="center">

# 🚿 logpipe

**Tail, filter and alert on log streams in real time — one binary, zero dependencies.**

[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/fmaterak/logpipe/ci.yml?style=flat-square&label=CI)](https://github.com/fmaterak/logpipe/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/fmaterak/logpipe?style=flat-square)](https://goreportcard.com/report/github.com/fmaterak/logpipe)

*When `grep` and `tail -f` aren't enough.*

</div>

```bash
logpipe tail /var/log/app.log \
    --filter "level=error" \
    --alert  "pattern=OOM,threshold=3,window=10s" \
    --output json
```

---

## Why logpipe?

- ⚡ **Fast** — concurrent, goroutine-based pipeline sustaining **1M+ msg/s** on a single core.
- 🎯 **Smart filtering** — match by JSON field or raw regex, no fragile `awk` chains.
- 🔔 **Pattern alerting** — fire when a pattern hits *N* times inside a sliding time window.
- 🧩 **Pluggable** — add custom filters and formatters by implementing one Go interface.
- 📦 **Zero dependencies** — a single static binary. Nothing to install, nothing to run.

## Install

```bash
go install github.com/fmaterak/logpipe@latest
```

Or grab a pre-built binary from [Releases](https://github.com/fmaterak/logpipe/releases).

## Quick start

```bash
# Only ERROR lines
logpipe tail app.log --filter "level=error"

# Regex on raw lines
logpipe tail app.log --filter-regex "timeout|connection refused"

# Alert: "OOM" 3+ times in 10s
logpipe tail app.log --alert "pattern=OOM,threshold=3,window=10s"

# Pipe-friendly (stdin), JSON output
kubectl logs -f my-pod | logpipe filter --filter "level=error" --output json

# Export matches to CSV
logpipe tail app.log --filter "level=error" --output csv > errors.csv
```

## How it works

A concurrent, stage-based pipeline where each stage runs in its own goroutine pool and
communicates over buffered channels — giving natural backpressure instead of dropped events.

```
[Source] → [Parser] → [Filter] → [Matcher] → [Output]
 file/stdin  JSON/raw  regex/field  sliding    text/JSON/CSV
                                    window
```

Pattern matching uses a fixed-size **ring buffer** per pattern for O(1) window counting with
zero heap allocations on the hot path.

## Benchmarks

Single core, Apple M2, Go 1.22, ~200-byte lines.

| Scenario | Throughput | p99 latency |
|---|---|---|
| Raw tail, no filter | **1.2M msg/s** | < 1 ms |
| Regex filter | 850K msg/s | ~1.2 ms |
| JSON field filter | 620K msg/s | ~1.8 ms |
| Full pipeline (filter + 3 alerts + JSON) | 310K msg/s | ~3.4 ms |

```bash
go test ./... -bench=. -benchmem   # reproduce
```

## Extend it

Implement one interface, register it, done:

```go
type Filter interface {
    Match(entry LogEntry) bool
    Name() string
}

filter.Register("my-filter", func(args map[string]string) filter.Filter {
    return &MyFilter{threshold: args["threshold"]}
})
```

## Motivation

Debugging distributed 5G RAN systems at Nokia, I kept re-chaining `tail | grep | awk` plus
throwaway Python. `logpipe` is that workflow, generalized into one composable tool — no full
observability stack required.

## Contributing

PRs welcome — please open an issue first for larger changes.

```bash
git clone https://github.com/fmaterak/logpipe && cd logpipe && go test ./...
```

## License

MIT — see [LICENSE](LICENSE).
