// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/demo"
)

func TestTheCommandLineReadsEveryFlag(t *testing.T) {
	t.Parallel()

	// Arrange
	args := []string{"-web", "-port", "13581", "-version", "v0.12.0", "-hold-ci", "-no-auto-announce"}
	want := demo.CommandLine{Web: true, Port: 13581, Version: "v0.12.0", HoldCI: true, NoAutoAnnounce: true}

	// Act
	asked, err := demo.ReadCommandLine(args, &bytes.Buffer{})

	// Assert
	if err != nil || asked != want {
		t.Errorf("ReadCommandLine(%q) = %+v, %v, want %+v", args, asked, err, want)
	}
}

func TestTheCommandLineRunsTheTerminalByDefault(t *testing.T) {
	t.Parallel()

	// Arrange
	want := demo.CommandLine{Version: "demo"}

	// Act
	asked, err := demo.ReadCommandLine(nil, &bytes.Buffer{})

	// Assert
	if err != nil || asked != want {
		t.Errorf("ReadCommandLine() = %+v, %v, want %+v", asked, err, want)
	}
}

func TestTheCommandLineRefusesAFlagItDoesNotKnow(t *testing.T) {
	t.Parallel()

	// Arrange
	var said bytes.Buffer

	// Act
	_, err := demo.ReadCommandLine([]string{"-webb"}, &said)

	// Assert
	if err == nil || errors.Is(err, flag.ErrHelp) {
		t.Fatalf("ReadCommandLine(-webb) = %v, want a refusal", err)
	}

	if !strings.Contains(said.String(), "-webb") {
		t.Errorf("ReadCommandLine(-webb) said %q, want it to name the flag", said.String())
	}
}

func TestTheCommandLineAsksForItsUsage(t *testing.T) {
	t.Parallel()

	// Act
	_, err := demo.ReadCommandLine([]string{"-h"}, &bytes.Buffer{})

	// Assert
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("ReadCommandLine(-h) = %v, want flag.ErrHelp", err)
	}
}

func TestTheCommandLineRefusesAnArgumentThatIsNoFlag(t *testing.T) {
	t.Parallel()

	// Arrange
	var said bytes.Buffer

	// Act
	_, err := demo.ReadCommandLine([]string{"-hold-ci", "web"}, &said)

	// Assert
	if !errors.Is(err, demo.ErrStrayArgument) {
		t.Fatalf("ReadCommandLine(-hold-ci web) = %v, want demo.ErrStrayArgument", err)
	}

	if !strings.Contains(said.String(), `"web"`) {
		t.Errorf("ReadCommandLine(-hold-ci web) said %q, want it to name the argument", said.String())
	}
}
