// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// errLefthookRefused stands in for a lefthook that could not dump its
// configuration.
var errLefthookRefused = errors.New("couldn't load config: yaml: line 3")

// errLefthookFoundNone is lefthook's refusal to dump where it finds no
// configuration.
var errLefthookFoundNone = errors.New(`couldn't load config: No config files with names ` +
	`["lefthook" ".lefthook" ".config/lefthook"] have been found in "/work"`)

// repositoryHolding is a directory holding each of files, every one a
// configuration lefthook could read.
func repositoryHolding(t *testing.T, files ...string) string {
	t.Helper()

	dir := t.TempDir()

	for _, file := range files {
		path := filepath.Join(dir, file)

		err := os.MkdirAll(filepath.Dir(path), 0o750)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(path, []byte("pre-commit:\n"), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

// configuredRepository is a directory lefthook would read a configuration
// from.
func configuredRepository(t *testing.T) string {
	t.Helper()

	return repositoryHolding(t, "lefthook.yml")
}

// dumping is a run that answers every command with dump, and records each
// command it was handed.
func dumping(dump string, handed *[]proc.Command) func(context.Context, proc.Command) ([]byte, error) {
	return func(_ context.Context, command proc.Command) ([]byte, error) {
		*handed = append(*handed, command)

		return []byte(dump), nil
	}
}

// readDump is what ReadConfigured makes of one dump, in a configured
// repository.
func readDump(t *testing.T, dump string) (hooks.Configured, error) {
	t.Helper()

	var handed []proc.Command

	return hooks.ReadConfigured(t.Context(), configuredRepository(t), dumping(dump, &handed))
}

// lefthooksDump is a merged configuration as lefthook dump prints it: its
// settings beside its hooks, each hook spelled one of the three ways lefthook
// takes, and one hook only git runs.
const lefthooksDump = `{
  "min_version": "1.4.2",
  "output": ["meta", "summary"],
  "colors": {"cyan": 14},
  "templates": {"dip": "docker compose run --rm"},
  "remotes": [{"git_url": "https://example.com/hooks", "configs": ["lefthook.yml"]}],
  "ai": {"claude": {"run": "x"}},
  "commit-msg": {"commands": {"conventional": {"run": "check {1}"}}},
  "pre-commit": {"parallel": true, "commands": {"lint": {"run": "golangci-lint run"}}},
  "pre-push": {"jobs": [{"name": "test", "run": "go test ./..."}]},
  "validate": {"scripts": {"validate.sh": {"runner": "sh"}}}
}`

func TestReadConfiguredOffersTheHooksARunByHandStandsInFor(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := configuredRepository(t)

	var handed []proc.Command

	// Act
	configured, err := hooks.ReadConfigured(t.Context(), dir, dumping(lefthooksDump, &handed))
	// Assert
	if err != nil {
		t.Fatalf("ReadConfigured: %v", err)
	}

	if len(handed) != 1 || handed[0].Dir != dir || !slices.Equal(handed[0].Args, hooks.DumpCommand(dir).Args) {
		t.Errorf("ran %+v, want lefthook dump in %s", handed, dir)
	}

	if want := []string{preCommit, prePush, "validate"}; !slices.Equal(configured.Runnable, want) {
		t.Errorf("Runnable = %q, want %q", configured.Runnable, want)
	}

	want := []hooks.GitOnly{{Name: commitMsg, Reason: "git hands it the file holding the commit message"}}
	if !slices.Equal(configured.GitOnly, want) {
		t.Errorf("GitOnly = %+v, want %+v", configured.GitOnly, want)
	}
}

func TestReadConfiguredKeepsGitsOrderThenTheConfigurationsOwnNames(t *testing.T) {
	t.Parallel()

	// Arrange
	dump := `{"zeta": {"jobs": [{"run": "z"}]}, "pre-push": {"jobs": [{"run": "p"}]},
		"alpha": {"jobs": [{"run": "a"}]}, "pre-commit": {"jobs": [{"run": "c"}]}}`

	// Act
	configured, err := readDump(t, dump)

	// Assert
	want := []string{preCommit, prePush, "alpha", "zeta"}
	if err != nil || !slices.Equal(configured.Runnable, want) {
		t.Errorf("ReadConfigured = %q, %v; want %q", configured.Runnable, err, want)
	}
}

func TestReadConfiguredLeavesOutAHookWithNothingToRun(t *testing.T) {
	t.Parallel()

	// Act
	configured, err := readDump(t, `{"post-commit": {"parallel": true}}`)

	// Assert
	if err != nil || len(configured.Runnable) != 0 || len(configured.GitOnly) != 0 {
		t.Errorf("ReadConfigured = %+v, %v; want nothing listed", configured, err)
	}
}

func TestReadConfiguredAsksLefthookWhereverItFindsAConfiguration(t *testing.T) {
	t.Parallel()

	for name, files := range map[string][]string{
		"a lefthook.jsonc":                         {"lefthook.jsonc"},
		"a local override alone":                   {localOverride},
		"no file, where LEFTHOOK_CONFIG names one": nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := repositoryHolding(t, files...)

			var handed []proc.Command

			// Act
			configured, err := hooks.ReadConfigured(t.Context(), dir, dumping(lefthooksDump, &handed))

			// Assert
			if err != nil || len(handed) != 1 || len(configured.Runnable) == 0 {
				t.Errorf("ReadConfigured = %+v, %v after %d dumps; want lefthook asked once and its hooks listed",
					configured, err, len(handed))
			}
		})
	}
}

func TestReadConfiguredFindsNoHookWhereLefthookFindsNoConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := repositoryHolding(t, "README.md")
	refusing := func(context.Context, proc.Command) ([]byte, error) { return nil, errLefthookFoundNone }

	// Act
	configured, err := hooks.ReadConfigured(t.Context(), dir, refusing)

	// Assert
	if err != nil || configured.Runnable != nil || configured.GitOnly != nil {
		t.Errorf("ReadConfigured = %+v, %v; want nothing and no error", configured, err)
	}
}

func TestEveryGitHookIsRunByHandOrSaysWhatGitHandsIt(t *testing.T) {
	t.Parallel()

	runByHand := map[string]bool{
		"applypatch-msg": false, "pre-applypatch": true, "post-applypatch": true, preCommit: true,
		"pre-merge-commit": true, "prepare-commit-msg": false, commitMsg: false, "post-commit": true,
		"pre-rebase": false, postCheckout: false, "post-merge": false, prePush: true,
		"pre-receive": false, "update": false, "proc-receive": false, "post-receive": false,
		"post-update": false, "reference-transaction": false, "push-to-checkout": false, "pre-auto-gc": true,
		"post-rewrite": false, "sendemail-validate": false, "fsmonitor-watchman": false, "p4-changelist": false,
		"p4-prepare-changelist": false, "p4-post-changelist": true, "p4-pre-submit": true,
		"post-index-change": false,
	}

	for hook, byHand := range runByHand {
		t.Run(hook, func(t *testing.T) {
			t.Parallel()

			// Act
			configured, err := readDump(t, `{"`+hook+`": {"jobs": [{"run": "true"}]}}`)
			// Assert
			if err != nil {
				t.Fatalf("ReadConfigured: %v", err)
			}

			if byHand && !slices.Equal(configured.Runnable, []string{hook}) {
				t.Errorf("Runnable = %q, want %s run by hand", configured.Runnable, hook)
			}

			if !byHand && (len(configured.GitOnly) != 1 || configured.GitOnly[0].Name != hook ||
				!strings.HasPrefix(configured.GitOnly[0].Reason, "git hands it ")) {
				t.Errorf("GitOnly = %+v, want %s listed with what git hands it", configured.GitOnly, hook)
			}
		})
	}
}

func TestReadConfiguredSaysWhyLefthookCouldNotDumpAConfigurationItReads(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"lefthook.yml", localOverride, ".config/lefthook-local.jsonc"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := repositoryHolding(t, file)
			refusing := func(context.Context, proc.Command) ([]byte, error) { return nil, errLefthookRefused }

			// Act
			_, err := hooks.ReadConfigured(t.Context(), dir, refusing)

			// Assert
			if !errors.Is(err, errLefthookRefused) || !strings.Contains(err.Error(), "reading lefthook's configuration") {
				t.Errorf("ReadConfigured = %v, want lefthook's refusal, said to be reading its configuration", err)
			}
		})
	}
}

func TestReadConfiguredRefusesWhatIsNotAConfiguration(t *testing.T) {
	t.Parallel()

	for name, dump := range map[string]string{
		"a version line": "lefthook 2.2.1",
		"a list":         "[]",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := readDump(t, dump)

			// Assert
			if !errors.Is(err, hooks.ErrUnreadableConfiguration) {
				t.Errorf("ReadConfigured(%q) = %v, want ErrUnreadableConfiguration", dump, err)
			}
		})
	}
}
