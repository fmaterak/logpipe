// Command logpipe is the CLI entrypoint. It dispatches to subcommands `tail`,
// `filter` and `version`. We deliberately use the standard library `flag`
// package instead of an external CLI framework to honour the README's
// "zero external dependencies" pledge.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	// Blank-import bundled plugins so they self-register before the CLI
	// dispatches. Users adding their own plugins do the same in their fork.
	_ "github.com/fmaterak/logpipe/plugins"
)

const usage = `logpipe - real-time log streaming, filtering and pattern alerting

Usage:
  logpipe <command> [flags]

Commands:
  tail      Tail a file (or stdin if no path is given) through the pipeline
  filter    Read from stdin only - convenience alias for ` + "`tail`" + ` without a path
  version   Print version information

Run "logpipe <command> -h" for command-specific flags.
`

var version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	// Hook SIGINT/SIGTERM to a context so the pipeline shuts down cleanly
	// instead of being killed mid-write. This matters for the file source
	// where output buffers must be flushed.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "tail":
		err = runTail(ctx, args)
	case "filter":
		err = runFilter(ctx, args)
	case "version", "-v", "--version":
		fmt.Printf("logpipe %s\n", version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "logpipe: %v\n", err)
		os.Exit(1)
	}
}
