// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
)

// noConfigWritten fails the test when a configuration file was written in dir.
func noConfigWritten(t *testing.T, dir string) {
	t.Helper()

	_, err := os.Stat(filepath.Join(dir, config.FileName))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a configuration file was written in %s (stat: %v)", dir, err)
	}
}

// answersThenEnds answers each question with the next of answers, then reads
// the end of the input, as a pipe that runs dry does.
func answersThenEnds(answers ...string) func(string) (string, error) {
	return func(string) (string, error) {
		if len(answers) == 0 {
			return "", io.EOF
		}

		next := answers[0]
		answers = answers[1:]

		return next, nil
	}
}

func TestConfigInitGlobalWithoutAHomeSaysSo(t *testing.T) {
	// Arrange
	where := place{dir: t.TempDir(), home: ""}

	// Act
	printed, err := runStreamsAt(t, where, unusedPrompt(t), "config", "init", "--template", "--global")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "determining the home directory") {
		t.Errorf("config init --global with no home = %v, want it to say it has no home to write to (%+v)",
			err, printed)
	}

	noConfigWritten(t, where.dir)
}

func TestConfigInitIntoADirectoryItCannotWriteSaysSo(t *testing.T) {
	cases := map[string]struct {
		args   []string
		prompt func(t *testing.T) cli.Prompt
	}{
		"a template": {
			args:   strings.Fields("config init --template"),
			prompt: unusedPrompt,
		},
		"the guided setup, every prompt skipped": {
			args:   strings.Fields("config init"),
			prompt: func(*testing.T) cli.Prompt { return scripted([]string{""}, []string{""}) },
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			if os.Geteuid() == 0 {
				t.Skip("root writes into a directory whatever its mode, so the save would succeed")
			}

			dir := t.TempDir()

			err := os.Chmod(dir, 0o500)
			if err != nil {
				t.Fatal(err)
			}

			t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

			// Act
			_, err = runGuided(t, dir, tt.prompt(t), tt.args...)

			// Assert
			if !errors.Is(err, fs.ErrPermission) || !strings.Contains(err.Error(), config.FileName) {
				t.Errorf("config init into a sealed directory = %v, want the file it could not write named", err)
			}

			noConfigWritten(t, dir)
		})
	}
}

func TestGuidedInitStopsWhenTheWebhookCannotBeRead(t *testing.T) {
	// Arrange
	// Jira is skipped, then reading the webhook fails.
	dir := t.TempDir()
	broken := cli.Prompt{
		Line:   func(string) (string, error) { return "", nil },
		Secret: func(string) (string, error) { return "", errPromptBroke },
	}

	// Act
	_, err := runGuided(t, dir, broken, "config", "init")

	// Assert
	if !errors.Is(err, errPromptBroke) {
		t.Errorf("config init returned %v, want the webhook read's failure surfaced", err)
	}

	noConfigWritten(t, dir)
}

func TestGuidedInitStopsWhenNothingAnswersWhetherToKeepAFailedCheck(t *testing.T) {
	// Arrange
	// The Jira check fails, and the input ends before the question of keeping
	// the credential anyway can be answered.
	dir := t.TempDir()
	badJira := jiraServer(t, http.StatusUnauthorized, "<html>login</html>", new(atomic.Bool)).URL
	prompt := cli.Prompt{
		Line:   answersThenEnds(badJira),
		Secret: answersThenEnds("bad-token"),
	}

	// Act
	_, err := runGuided(t, dir, prompt, "config", "init")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "no terminal to answer on") {
		t.Errorf("config init = %v, want it to stop at the question nothing could answer", err)
	}

	wantExit(t, err, 2)
	noConfigWritten(t, dir)
}

func TestGuidedInitStopsWhenNothingAnswersTheKeychainOffer(t *testing.T) {
	// Arrange
	// The Jira check passes, and the input ends before the offer to keep the
	// token in the keychain can be answered.
	dir := t.TempDir()

	var stored atomic.Bool

	prompt := cli.Prompt{
		Line:   answersThenEnds(workingJira(t)),
		Secret: answersThenEnds(guidedToken),
		StoreSecret: func(string) (string, error) {
			stored.Store(true)

			return "", nil
		},
	}

	// Act
	_, err := runGuided(t, dir, prompt, "config", "init")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "no terminal to answer on") || stored.Load() {
		t.Errorf("config init = %v, keychain used: %v; want it to stop at the offer, storing nothing",
			err, stored.Load())
	}

	wantExit(t, err, 2)
	noConfigWritten(t, dir)
}

func TestConfigInitRefusesALinkWhereTheFileWouldGo(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	elsewhere := filepath.Join(t.TempDir(), "elsewhere.json")

	err := os.Symlink(elsewhere, filepath.Join(dir, config.FileName))
	if err != nil {
		t.Fatalf("linking the file's path elsewhere: %v", err)
	}

	// Act
	output, err := run(t, dir, "config", "init", "--template")

	// Assert
	_, statErr := os.Lstat(elsewhere)
	if err == nil || !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("config init over a dangling link = %v (%s), and its target %v; want it refused, nothing written",
			err, output, statErr)
	}
}

func TestGuidedInitWithNothingToReadPointsToTheTemplate(t *testing.T) {
	cases := map[string]cli.Prompt{
		"the address": {
			Line:   answersThenEnds(),
			Secret: answersThenEnds(),
		},
		"the token": {
			Line:   answersThenEnds("https://jira.example.com"),
			Secret: answersThenEnds(),
		},
		"the webhook": {
			Line:   answersThenEnds(""),
			Secret: answersThenEnds(),
		},
	}

	for name, prompt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			dir := t.TempDir()

			// Act
			_, err := runGuided(t, dir, prompt, "config", "init")

			// Assert
			if err == nil || !strings.Contains(err.Error(), "pass --template") {
				t.Errorf("config init with %s unread = %v, want it to name --template", name, err)
			}

			wantExit(t, err, 2)
			noConfigWritten(t, dir)
		})
	}
}

// outsideRepository is what a file outside a repository, which the
// repository's configuration file links to, holds before config init runs.
const outsideRepository = "export PATH=$HOME/bin:$PATH\n"

// linkedRepositoryFile is a repository whose configuration file is a link to
// a file outside it holding outsideRepository, with no home file, and that
// file.
func linkedRepositoryFile(t *testing.T) (string, string) {
	t.Helper()

	repo := t.TempDir()

	err := os.Mkdir(filepath.Join(repo, ".git"), 0o700)
	if err != nil {
		t.Fatalf("making the repository: %v", err)
	}

	target := filepath.Join(t.TempDir(), "profile")

	err = os.WriteFile(target, []byte(outsideRepository), 0o600)
	if err != nil {
		t.Fatalf("writing the file outside the repository: %v", err)
	}

	err = os.Symlink(target, filepath.Join(repo, config.FileName))
	if err != nil {
		t.Fatalf("linking the repository's file outside it: %v", err)
	}

	return repo, target
}

func TestConfigInitForceRefusesARepositoryFileThatIsALink(t *testing.T) {
	// Arrange
	repo, target := linkedRepositoryFile(t)

	// Act
	output, err := run(t, repo, "config", "init", "--template", "--force")

	// Assert
	if !errors.Is(err, config.ErrLinkedFile) {
		t.Errorf("config init --template --force through a link = %v (%s), want ErrLinkedFile", err, output)
	}

	unchanged(t, target, outsideRepository)
}

func TestGuidedInitForceRefusesARepositoryFileThatIsALinkBeforeAsking(t *testing.T) {
	// Arrange
	repo, target := linkedRepositoryFile(t)

	// Act
	output, err := run(t, repo, "config", "init", "--force")

	// Assert
	if !errors.Is(err, config.ErrLinkedFile) {
		t.Errorf("config init --force through a link = %v (%s), want ErrLinkedFile before any question", err, output)
	}

	unchanged(t, target, outsideRepository)
}
