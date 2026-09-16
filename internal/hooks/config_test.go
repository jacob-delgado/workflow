// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/hooks"
)

// errNoConfig stands in for lefthook failing to read its configuration.
var errNoConfig = errors.New("lefthook: no config")

// dumping is a runner that answers lefthook dump with out.
func dumping(t *testing.T, out string, err error) hooks.Runner {
	t.Helper()

	return func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "lefthook" || !slices.Equal(args, []string{"dump", "--format", "json"}) {
			t.Errorf("ran %s %q, want lefthook dump --format json", name, args)
		}

		return []byte(out), err
	}
}

func TestReadConfigListsEachHooksJobs(t *testing.T) {
	t.Parallel()

	// The dump's shape as lefthook 2.1 writes it for this repository, with a
	// setting that is not a hook and each of the three ways to declare jobs.
	dump := `{
	  "colors": false,
	  "pre-commit": {"parallel": true, "commands": {"lint": {"run": "x"}, "fmt": {"run": "y"}}},
	  "commit-msg": {"scripts": {"check.sh": {"runner": "bash"}}},
	  "pre-push": {"jobs": [{"name": "test", "run": "go test"}]},
	  "post-merge": "not an object"
	}`

	configured, err := hooks.ReadConfig(t.Context(), dumping(t, dump, nil))
	if err != nil {
		t.Fatalf("ReadConfig returned %v, want nil", err)
	}

	want := map[string][]string{
		preCommit: {"fmt", "lint"},
		commitMsg: {"check.sh"},
		prePush:   {"test"},
	}

	if len(configured) != len(want) {
		t.Errorf("ReadConfig = %v, want %v", configured, want)
	}

	for hook, jobs := range want {
		if !slices.Equal(configured[hook], jobs) {
			t.Errorf("%s jobs = %q, want %q", hook, configured[hook], jobs)
		}
	}
}

func TestReadConfigReportsFailures(t *testing.T) {
	t.Parallel()

	_, err := hooks.ReadConfig(t.Context(), dumping(t, "", errNoConfig))
	if !errors.Is(err, errNoConfig) {
		t.Errorf("ReadConfig returned %v, want lefthook's error", err)
	}

	_, err = hooks.ReadConfig(t.Context(), dumping(t, "{not json", nil))
	if err == nil {
		t.Error("ReadConfig accepted output that is not JSON")
	}
}

func TestRunCommandRunsOneHookWithoutATerminal(t *testing.T) {
	t.Parallel()

	command := hooks.RunCommand("/work", preCommit)
	if command.Dir != "/work" || command.Name != "lefthook" ||
		!slices.Equal(command.Args, []string{"run", preCommit, "--no-tty"}) {
		t.Errorf("RunCommand = %+v", command)
	}
}
