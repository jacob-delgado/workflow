// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"
)

func TestScriptableCommandsLeadTheirHelpWithExamples(t *testing.T) {
	for command := range strings.FieldsSeq("status reviews repositories summary branch pr announce comment doctor") {
		t.Run(command, func(t *testing.T) {
			// Act
			output, err := run(t, t.TempDir(), command, "--help")
			// Assert
			if err != nil {
				t.Fatalf("%s --help: %v", command, err)
			}

			_, examples, found := strings.Cut(output, "Examples:")
			if !found || !strings.Contains(examples, command) {
				t.Errorf("%s --help has no example of itself:\n%s", command, output)
			}
		})
	}
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
