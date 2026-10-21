// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// CommandLine is what the demo's command line asks of it: the web interface
// rather than the terminal's, served on the loopback Port, 0 for one the
// system chooses, and naming the build Version, and the world made with
// HoldCI and AutoAnnounceOff.
type CommandLine struct {
	Web            bool
	Port           int
	Version        string
	HoldCI         bool
	NoAutoAnnounce bool
}

// ErrStrayArgument is a command line holding an argument that is no flag,
// as `demo web` with its dash forgotten.
var ErrStrayArgument = errors.New("an argument that is no flag")

// ReadCommandLine reads args, the demo's command line, or fails as the flag
// package does, having said why on said: a flag it does not know, -h asking
// for its usage (flag.ErrHelp), or an argument that is no flag
// (ErrStrayArgument), which would otherwise be dropped and the terminal run.
func ReadCommandLine(args []string, said io.Writer) (CommandLine, error) {
	var asked CommandLine

	set := flag.NewFlagSet("demo", flag.ContinueOnError)
	set.SetOutput(said)
	set.BoolVar(&asked.Web, "web", false, "serve the web interface instead of the terminal's")
	set.IntVar(&asked.Port, "port", 0, "the loopback port -web serves on; 0 for one the system chooses")
	set.StringVar(&asked.Version, "version", "demo", "the version the web page names the build")
	set.BoolVar(&asked.HoldCI, "hold-ci", false, "keep the end-to-end check running for good")
	set.BoolVar(&asked.NoAutoAnnounce, "no-auto-announce", false,
		"arm no announcement when a pull request opens (messaging.auto_announce off)")

	err := set.Parse(args)
	if err != nil {
		return CommandLine{}, fmt.Errorf("reading the flags: %w", err)
	}

	if set.NArg() > 0 {
		fmt.Fprintf(said, "%q is no flag; a flag starts with a dash\n", set.Arg(0))
		set.Usage()

		return CommandLine{}, fmt.Errorf("reading the flags: %w: %q", ErrStrayArgument, set.Arg(0))
	}

	return asked, nil
}
