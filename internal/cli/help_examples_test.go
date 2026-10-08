// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli"
)

func TestEveryCommandLeadsItsHelpWithExamples(t *testing.T) {
	for _, path := range leafCommands(cli.NewRootCmd(cli.Prompt{})) {
		t.Run(path, func(t *testing.T) {
			// Act
			output, err := run(t, t.TempDir(), append(strings.Fields(path)[1:], "--help")...)
			// Assert
			if err != nil {
				t.Fatalf("%s --help: %v", path, err)
			}

			_, examples, found := strings.Cut(output, "Examples:")
			if !found || !strings.Contains(examples, path) {
				t.Errorf("%s --help has no example of itself:\n%s", path, output)
			}
		})
	}
}

// leafCommands is the path of every command under root that has none under it,
// the commands a person runs rather than the groups that hold them.
func leafCommands(root *cobra.Command) []string {
	var paths []string

	for _, child := range root.Commands() {
		if child.HasSubCommands() {
			paths = append(paths, leafCommands(child)...)

			continue
		}

		paths = append(paths, child.CommandPath())
	}

	return paths
}

func TestASynopsisThatSaysFailsNamesTheExitStatus(t *testing.T) {
	cases := map[string]struct {
		args string
		want string
	}{
		"status, outside a repository": {args: "status --help", want: "exits 4"},
		"config show, with no file":    {args: "config show --help", want: "exits 3"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			output, err := run(t, t.TempDir(), strings.Fields(tt.args)...)

			// Assert
			if err != nil || !strings.Contains(output, tt.want) {
				t.Errorf("%s = %v, want the synopsis to say %q:\n%s", tt.args, err, tt.want, output)
			}
		})
	}
}
