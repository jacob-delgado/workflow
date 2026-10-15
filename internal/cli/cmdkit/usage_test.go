// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
)

func TestAMistakeInTheCallIsAUsageErrorPointingAtTheHelp(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args  []string
		hint  string
		cause string
	}{
		"a command misspelled": {
			args: []string{"prob"}, hint: "Did you mean this?\n\tprobe\n\nRun 'workflow --help' for usage.",
			cause: `unknown command "prob"`,
		},
		"a command nothing is near": {
			args: []string{"launch"}, hint: "Run 'workflow --help' for usage.", cause: `unknown command "launch"`,
		},
		"a flag the command does not take": {
			args: []string{probeCommand, "--nope"}, hint: "Run 'workflow probe --help' for usage.",
			cause: "unknown flag: --nope",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			env, _ := inATempDir(t)
			root := probeTree(env, func(*cobra.Command, []string) error { return nil })

			// Act
			_, err := runTree(t, root, tt.args...)

			// Assert
			// The hint is the whole of what follows cobra's words, so a
			// suggestion where there is none is as wrong as one left out.
			cause, hint, _ := strings.Cut(err.Error(), "\n\n")
			if !errors.Is(err, cmdkit.ErrUsage) || !strings.HasPrefix(cause, tt.cause) || hint != tt.hint {
				t.Errorf("%s = %v, want a usage error saying %q, then only %q", name, err, tt.cause, tt.hint)
			}
		})
	}
}

func TestACallTheCommandTakesIsNoUsageError(t *testing.T) {
	t.Parallel()

	// Arrange
	env, _ := inATempDir(t)
	ran := false
	root := probeTree(env, func(*cobra.Command, []string) error {
		ran = true

		return nil
	})

	// Act
	_, err := runTree(t, root, probeCommand, "anything")

	// Assert
	if err != nil || !ran {
		t.Errorf("probe anything = %v, ran %v; want it run", err, ran)
	}
}

func TestACommandThatOnlyGroupsOthersPrintsItsHelp(t *testing.T) {
	t.Parallel()

	// Arrange
	root := groupTree(t)

	// Act
	printed, err := runTree(t, root, "group")

	// Assert
	if err != nil || !strings.Contains(printed.Stdout, "Group the probes") || !strings.Contains(printed.Stdout, "member") {
		t.Errorf("group = %v, printed %q; want its help, naming what it groups", err, printed.Stdout)
	}
}

func TestACommandThatOnlyGroupsOthersRefusesOneItDoesNotHold(t *testing.T) {
	t.Parallel()

	// Arrange
	root := groupTree(t)

	// Act
	printed, err := runTree(t, root, "group", "bogus")

	// Assert
	// Left without a run of its own, the group would print its help for the
	// command it does not hold and succeed, before its check could refuse it.
	if !errors.Is(err, cmdkit.ErrUsage) || !strings.Contains(err.Error(), `"bogus"`) || printed.Stdout != "" {
		t.Errorf("group bogus = %v, printed %q; want a usage error naming bogus, and no help", err, printed.Stdout)
	}
}

// groupTree is probeTree with a command, group, that only groups another,
// member, and takes no arguments of its own, as cobra's completion does.
func groupTree(t *testing.T) *cobra.Command {
	t.Helper()

	env, _ := inATempDir(t)
	root := probeTree(env, func(*cobra.Command, []string) error { return nil })
	group := &cobra.Command{Use: "group", Short: "Group the probes", Args: cobra.NoArgs}
	group.AddCommand(&cobra.Command{Use: "member", RunE: func(*cobra.Command, []string) error { return nil }})
	root.AddCommand(group)
	// probeTree marked its tree before group joined it.
	cmdkit.MarkMisuse(group)

	return root
}
