// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Command demo runs the terminal interface over a made-up day's work, for the
// docs' recordings:
//
//	go run ./cmd/demo
//
// Nothing it shows is real and nothing it does leaves the process. The world
// and its tests are internal/demo's; no release builds this command.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jacob-delgado/workflow/internal/demo"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	status := run(ctx)

	stop()
	os.Exit(status)
}

// interrupted is the status an interrupted program exits with, as the
// shell's 128 plus SIGINT's 2 and as workflow's own command line.
const interrupted = 130

// run runs the interface over the demo's world until it quits or a signal
// ends ctx, the world's clock moving on as the program runs, and says how it
// ended.
func run(ctx context.Context) int {
	began := time.Now()
	world := demo.New(func() time.Duration { return time.Since(began) })

	_, err := tui.Run(ctx, tui.New(world.Config(), nil, world.Deps()), os.Stdin, os.Stdout)

	switch {
	case ctx.Err() != nil:
		return interrupted
	case err != nil:
		fmt.Fprintln(os.Stderr, "demo:", err)

		return 1
	default:
		return 0
	}
}
