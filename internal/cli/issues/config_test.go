// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package issues_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/config"
)

func TestEveryIssueCommandRefusesAConfigurationItCannotRead(t *testing.T) {
	t.Parallel()

	// A file that exists but does not parse is not replaced by the defaults: a
	// command run on settings nobody chose would act on the wrong things.
	cases := [][]string{
		strings.Fields("issues"),
		strings.Fields("issue PROJ-1"),
		strings.Fields("transition PROJ-1 Done --dry-run"),
		strings.Fields("assign PROJ-1 bo --dry-run"),
		strings.Fields("worklog PROJ-1 1h --dry-run"),
	}

	for _, args := range cases {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			clitest.WriteFile(t, dir, "{not json")

			// Act
			_, err := clitest.Run(t, dir, args...)

			// Assert
			if !errors.Is(err, config.ErrInvalid) {
				t.Errorf("%s with a malformed configuration = %v, want it refused", name, err)
			}

			wantExit(t, err, 3)
		})
	}
}
