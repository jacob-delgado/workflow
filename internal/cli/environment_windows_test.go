// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"
)

// Windows reads a path rooted on the current drive (\logs) and one relative to
// another drive's own directory (D:logs) from somewhere other than the working
// directory, though neither is absolute: joined onto the working directory, it
// would name a place that is not the one asked for.
//
// Trade-off TRADE-35: only a Windows go test runs this, and no CI job is one.
func TestALogWindowsReadsFromElsewhereIsNotReadFromTheWorkingDirectory(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"rooted on the current drive":         `\workflow-no-such-directory\requests.log`,
		"relative to a drive's own directory": `Z:requests.log`,
	}

	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			printed, err := runFromARemovedDirectory(t, t.TempDir(), logOption, path, doctorCommand)

			// Assert
			if err != nil && strings.HasPrefix(err.Error(), "workflow: opening the request log: "+workdirUnread) {
				t.Errorf("--log %s from a removed directory = %v, want the log not read from the working "+
					"directory (%+v)", path, err, printed)
			}
		})
	}
}
