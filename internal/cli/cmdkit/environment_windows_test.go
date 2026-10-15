// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit_test

import (
	"strings"
	"testing"
)

// Windows reads a path rooted on the current drive (\logs) and one relative to
// another drive's own directory (D:logs) from somewhere other than the working
// directory, though neither is absolute: joined onto the working directory, it
// would name a place that is not the one asked for. So a request log named
// either way is opened as named, with no working directory to read.
//
// Trade-off TRADE-35: only a Windows go test runs this, and no CI job is one.
func TestARequestLogWindowsReadsFromElsewhereNeedsNoWorkingDirectory(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"rooted on the current drive":         `\workflow-no-such-directory\requests.log`,
		"relative to a drive's own directory": `Z:requests.log`,
	}

	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var opened bool

			root := requestLogRoot(fromARemovedDirectory(t), &opened)

			// Act
			_, err := runTree(t, root, "--request-log", path, probeCommand)

			// Assert
			// Where the file cannot be made, as where the directory or the
			// drive is missing, the open still names the path as given.
			openedAsNamed := opened ||
				err != nil && strings.HasPrefix(err.Error(), "opening the request log: open "+path+": ")
			if !openedAsNamed {
				t.Errorf("--request-log %s from a removed directory = %v, a log kept: %v; want it opened as "+
					"named, not read from the working directory", path, err, opened)
			}
		})
	}
}
