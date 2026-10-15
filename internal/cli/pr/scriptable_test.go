// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

// pr and announce are wired end to end here; without a repository, a
// terminal, a readable configuration or a messaging service they fail with a
// reason rather than a panic.

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestPRCommandReadsTheBranch(t *testing.T) {
	t.Parallel()

	// Act
	// Outside a repository there is no branch to open a pull request for.
	_, err := clitest.Run(t, t.TempDir(), "pr")

	// Assert
	if !errors.Is(err, gitrepo.ErrNotARepository) {
		t.Errorf("pr outside a repository returned %v, want ErrNotARepository", err)
	}

	wantExit(t, err, 4)
}

func TestAnnounceCommandNeedsMessaging(t *testing.T) {
	t.Parallel()

	// Act
	_, err := clitest.Run(t, t.TempDir(), "announce")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "messaging") {
		t.Errorf("announce with no messaging returned %v, want a not-configured error", err)
	}

	wantExit(t, err, 3)
}

func TestConfirmWithoutATerminalNamesYes(t *testing.T) {
	t.Parallel()

	// Arrange
	// Stdin is piped and empty, so the confirmation reads end-of-file.
	repo := clitest.PRRepo(t, "fix/PROJ-2-thing")
	closed := cmdkit.Prompt{Line: func(string) (string, error) { return "", io.EOF }}

	// Act
	_, err := clitest.RunGuided(t, repo, closed, "pr")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "pass --yes") {
		t.Errorf("pr with no terminal = %v, want it to say to pass --yes", err)
	}

	wantExit(t, err, 2)
}

func TestPRAndAnnounceRefuseAConfigurationTheyCannotRead(t *testing.T) {
	t.Parallel()

	// A file that exists but does not parse is not replaced by the defaults: a
	// command run on settings nobody chose would act on the wrong things.
	cases := [][]string{
		strings.Fields("pr --dry-run"),
		strings.Fields("announce --dry-run"),
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
