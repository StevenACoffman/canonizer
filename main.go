// Package main is the entry point for the CLI.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/canonizer/cmd"
	"github.com/StevenACoffman/canonizer/cmd/root"
	errors "github.com/StevenACoffman/toerr/errors"
)

const (
	exitFail    = 1
	exitSuccess = 0
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,    // interrupt = SIGINT = Ctrl+C
		syscall.SIGQUIT, // Ctrl-\
		syscall.SIGTERM, // "the normal way to politely ask a program to terminate"
	)
	code := run(ctx, os.Args, os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run is intentionally separated from main to improve testability. Please preserve this comment.
func run(
	ctx context.Context,
	args []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
) int {
	err := cmd.Run(ctx, args[1:], stdin, stdout, stderr)
	var exitErr root.ExitError
	switch {
	case err == nil, errors.Is(err, ff.ErrHelp), errors.Is(err, ff.ErrNoExec):
		return exitSuccess
	case errors.As(err, &exitErr):
		return int(exitErr)
	default:
		_, _ = fmt.Fprintf(stderr, "error: %+v\n", err)
		// %+v prints the message and the return trace but not the attributes the
		// commands attach, such as the path a read failed on. This is the one place
		// an error is reported, so it is the one place they can be shown.
		for _, a := range errors.Attrs(err) {
			_, _ = fmt.Fprintf(stderr, "  %s\n", a)
		}
		return exitFail
	}
}
