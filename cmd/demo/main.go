// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Command demo runs the terminal interface, or the --web server, over a
// made-up day's work, for the docs' recordings:
//
//	go run ./cmd/demo [-no-auto-announce]
//	go run ./cmd/demo -web [-port 0] [-version v0.12.0] [-hold-ci] [-no-auto-announce]
//
// With -web it serves the web interface on the loopback port -port names, 0
// for one the system chooses, and says where on stderr as workflow --web
// does, session and all; -version is what the page names the build, and
// -hold-ci keeps the end-to-end check running for good. -no-auto-announce,
// on either interface, turns messaging.auto_announce off, so opening a pull
// request arms no announcement. Nothing it shows is real and nothing it does
// leaves the process, but for the web page's configuration file, which a
// Settings save writes into a temporary directory removed on exit. The world
// and its tests are internal/demo's; no release builds this command.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/demo"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	status := run(ctx, os.Args[1:])

	stop()
	os.Exit(status)
}

// The statuses the demo exits with besides 0 and 1: the shell's 128 plus
// SIGINT's 2 when interrupted, as workflow's own command line, and 2 for
// flags it cannot read, as the flag package's own.
const (
	interrupted = 130
	misused     = 2
)

// readLatency is how long the world's slow reads take, as a remote's would:
// long enough that the recording shows each "reading" state for a beat, not
// a single frame.
const readLatency = 500 * time.Millisecond

// pushLatency is how long a push takes once it has printed, as a remote's
// answer would: long enough that the recording shows the push's output, its
// hooks passing and the branch sent, before the Branch pane says it is
// pushed.
const pushLatency = 1200 * time.Millisecond

// flags is what the command line asks of the demo.
type flags struct {
	web            bool
	port           int
	version        string
	holdCI         bool
	noAutoAnnounce bool
}

// parse reads args into flags, or fails as the flag package does, having
// said why on stderr.
func parse(args []string) (flags, error) {
	var asked flags

	set := flag.NewFlagSet("demo", flag.ContinueOnError)
	set.BoolVar(&asked.web, "web", false, "serve the web interface instead of the terminal's")
	set.IntVar(&asked.port, "port", 0, "the loopback port -web serves on; 0 for one the system chooses")
	set.StringVar(&asked.version, "version", "demo", "the version the web page names the build")
	set.BoolVar(&asked.holdCI, "hold-ci", false, "keep the end-to-end check running for good")
	set.BoolVar(&asked.noAutoAnnounce, "no-auto-announce", false,
		"arm no announcement when a pull request opens (messaging.auto_announce off)")

	err := set.Parse(args)
	if err != nil {
		return flags{}, fmt.Errorf("reading the flags: %w", err)
	}

	return asked, nil
}

// run runs the interface the flags in args ask for over the demo's world
// until it quits or a signal ends ctx, the world's clock moving on as the
// program runs, and says how it ended.
func run(ctx context.Context, args []string) int {
	asked, err := parse(args)

	switch {
	case errors.Is(err, flag.ErrHelp):
		return 0
	case err != nil:
		return misused
	}

	began := time.Now()
	world := demo.New(func() time.Duration { return time.Since(began) }, worldOptions(asked)...)

	return ended(ctx, serve(ctx, world, asked, os.Stderr))
}

// worldOptions is how the flags make the demo's world other than its
// default, with the latencies a recording wants.
func worldOptions(asked flags) []demo.Option {
	options := []demo.Option{demo.ReadLatency(readLatency), demo.PushLatency(pushLatency)}
	if asked.holdCI {
		options = append(options, demo.HoldCI())
	}

	if asked.noAutoAnnounce {
		options = append(options, demo.AutoAnnounceOff())
	}

	return options
}

// serve runs the web server over world when asked, saying where on notes,
// and the terminal interface otherwise.
func serve(ctx context.Context, world *demo.World, asked flags, notes io.Writer) error {
	if !asked.web {
		_, err := tui.Run(ctx, tui.New(world.Config(), nil, world.Deps()), os.Stdin, os.Stdout)

		return err
	}

	return serveWeb(ctx, world, asked, notes)
}

// serveWeb runs the web server over world, saying where on notes, with its
// configuration file kept in a directory of its own, removed once it stops.
func serveWeb(ctx context.Context, world *demo.World, asked flags, notes io.Writer) error {
	configDir, err := os.MkdirTemp("", "workflow-demo-")
	if err != nil {
		return fmt.Errorf("making the web demo's configuration directory: %w", err)
	}

	web := world.Web(asked.version, configDir)
	served := cli.WebServerAt(webserver.LoopbackAddr(asked.port))(ctx, web.Config, web.Deps, web.Info, notes)

	return errors.Join(served, os.RemoveAll(configDir))
}

// ended is the status a run that ended with err exits with.
func ended(ctx context.Context, err error) int {
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
