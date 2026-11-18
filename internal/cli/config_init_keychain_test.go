// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/config"
)

// recordingKeychain is a keychain that only notes that it was asked to keep
// a secret, for a test of whether setup touched it.
func recordingKeychain(stored *atomic.Bool) func(context.Context, string, string) error {
	return func(context.Context, string, string) error {
		stored.Store(true)

		return nil
	}
}

func TestGuidedInitKeepsNothingInTheKeychainWhenTheWebhookCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// The Jira check passes and the keychain is chosen, then the input ends
	// before the webhook is answered, so no file is written.
	dir := t.TempDir()

	var stored atomic.Bool

	prompt := cmdkit.Prompt{
		Line:        clitest.AnswersThenEnds(clitest.WorkingJira(t), "y"),
		Secret:      clitest.AnswersThenEnds(guidedToken),
		StoreSecret: recordingKeychain(&stored),
	}

	// Act
	_, err := clitest.RunGuided(t, dir, prompt, "config", "init")

	// Assert
	if err == nil || stored.Load() {
		t.Errorf("config init = %v, keychain replaced: %t; want the setup refused and the keychain as it was",
			err, stored.Load())
	}

	noConfigWritten(t, dir)
}

func TestGuidedInitKeepsNothingInTheKeychainWhenTheFileCannotBeWritten(t *testing.T) {
	t.Parallel()

	// Arrange
	// The home file beneath changes while the keychain offer is answered, so
	// the repository's file over it is refused.
	where := clitest.HomeAndRepository(t)
	homeFile := filepath.Join(where.Home, config.FileName)
	lines := clitest.AnswersThenEnds(clitest.WorkingJira(t), "y")

	var stored atomic.Bool

	prompt := cmdkit.Prompt{
		Line: func(question string) (string, error) {
			if strings.Contains(question, "keychain") {
				clitest.BreakFile(t, homeFile)()
			}

			return lines(question)
		},
		Secret:      clitest.Scripted(nil, []string{guidedToken, ""}).Secret,
		StoreSecret: recordingKeychain(&stored),
	}

	// Act
	_, err := clitest.RunStreamsAt(t, where, prompt, "config", "init")

	// Assert
	if !errors.Is(err, config.ErrChangedOnDisk) || stored.Load() {
		t.Errorf("config init = %v, keychain replaced: %t; want the write refused for the change and the "+
			"keychain as it was", err, stored.Load())
	}

	noConfigWritten(t, where.Dir)
}

func TestGuidedInitLeavesNoFileWhenTheKeychainCannotKeepTheToken(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	prompt := clitest.Scripted([]string{clitest.WorkingJira(t), "y"}, []string{guidedToken, ""})
	prompt.StoreSecret = func(context.Context, string, string) error { return errPromptBroke }

	// Act
	_, err := clitest.RunGuided(t, dir, prompt, "config", "init")

	// Assert
	if !errors.Is(err, errPromptBroke) {
		t.Errorf("config init = %v, want the keychain's failure surfaced", err)
	}

	_, statErr := os.Stat(filepath.Join(dir, config.FileName))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("a file reading its token from a keychain that does not hold it was left (stat: %v)", statErr)
	}
}

func TestGuidedInitOffersNoKeychainForAnEmptyToken(t *testing.T) {
	t.Parallel()

	// Arrange
	// The Jira check fails for the empty token, which is saved anyway.
	badJira := clitest.JiraServer(t, http.StatusUnauthorized, "<html>login</html>", new(atomic.Bool)).URL
	lines := clitest.Scripted([]string{badJira, "y"}, nil).Line

	var (
		asked  []string
		stored atomic.Bool
	)

	prompt := cmdkit.Prompt{
		Line: func(question string) (string, error) {
			asked = append(asked, question)

			return lines(question)
		},
		Secret:      clitest.Scripted(nil, []string{"", ""}).Secret,
		StoreSecret: recordingKeychain(&stored),
	}

	// Act
	_, err := clitest.RunGuided(t, t.TempDir(), prompt, "config", "init")

	// Assert
	if err != nil || stored.Load() || strings.Contains(strings.Join(asked, "\n"), "keychain") {
		t.Errorf("config init = %v, keychain used %t, asked %q; want no keychain offered with no token to keep",
			err, stored.Load(), asked)
	}
}

// refusingKeychainSetup answers a guided setup that checks a working Jira,
// keeps its token in a keychain that refuses it, and leaves the webhook blank.
func refusingKeychainSetup(t *testing.T) cmdkit.Prompt {
	t.Helper()

	prompt := clitest.Scripted([]string{clitest.WorkingJira(t), "y"}, []string{guidedToken, ""})
	prompt.StoreSecret = func(context.Context, string, string) error { return errPromptBroke }

	return prompt
}

// heldBytes is what the file at path holds, failing the test where it holds
// nothing.
func heldBytes(t *testing.T, path string) string {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	return string(contents)
}

func TestConfigInitForceWhoseKeychainRefusesGivesBackTheFileItReplaced(t *testing.T) {
	t.Parallel()

	// Arrange
	// The home file is a dotfiles checkout's link, which --force replaces
	// where it points.
	home, dotfiles := t.TempDir(), t.TempDir()
	old := `{"jira": {"base_url": "https://jira.example.com", "token": "old"}}` + "\n"
	target := filepath.Join(dotfiles, "workflow.json")
	homeFile := filepath.Join(home, config.FileName)

	clitest.WriteFile(t, dotfiles, old)

	err := errors.Join(os.Rename(filepath.Join(dotfiles, config.FileName), target), os.Symlink(target, homeFile))
	if err != nil {
		t.Fatalf("linking the home file into the dotfiles: %v", err)
	}

	where := clitest.Place{Dir: t.TempDir(), Home: home}

	// Act
	_, err = clitest.RunStreamsAt(t, where, refusingKeychainSetup(t), "config", "init", "--force", "--global")

	// Assert
	info, statErr := os.Lstat(homeFile)
	if !errors.Is(err, errPromptBroke) || statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("config init = %v, the home file %v; want the keychain's refusal and the link kept", err, statErr)
	}

	if held := heldBytes(t, target); held != old {
		t.Errorf("the dotfiles' file holds %q; want what it held before the setup, %q", held, old)
	}
}

func TestConfigInitForceOverARepositoryFileWhoseKeychainRefusesGivesItBack(t *testing.T) {
	t.Parallel()

	// Arrange
	where := clitest.HomeAndRepository(t)
	old := `{"jira": {"project": "OLD"}}` + "\n"
	clitest.WriteFile(t, where.Dir, old)

	// Act
	_, err := clitest.RunStreamsAt(t, where, refusingKeychainSetup(t), "config", "init", "--force")

	// Assert
	if !errors.Is(err, errPromptBroke) {
		t.Fatalf("config init = %v, want the keychain's refusal", err)
	}

	if held := heldBytes(t, filepath.Join(where.Dir, config.FileName)); held != old {
		t.Errorf("the repository's file holds %q; want what it held before the setup, %q", held, old)
	}
}
