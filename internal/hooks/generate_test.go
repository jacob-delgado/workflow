// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"go.yaml.in/yaml/v3"

	"github.com/jacob-delgado/workflow/internal/hooks"
)

// executable is a hook git would run.
func executable(script string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(script), Mode: 0o755}
}

// simplePreCommit is a hook of plain commands, which lefthook can run as jobs.
const simplePreCommit = `#!/bin/sh
# Format check, then vet.
set -e

gofmt -l .
go vet ./...
`

// complexCommitMsg reads its argument and branches, which only a script can do.
const complexCommitMsg = `#!/usr/bin/env bash
if ! grep -qE '^(feat|fix)' "$1"; then
  echo "bad subject"
  exit 1
fi
`

func TestExistingHooksAreTheOnesGitWouldRun(t *testing.T) {
	t.Parallel()

	dir := fstest.MapFS{
		preCommit:                executable(simplePreCommit),
		commitMsg:                executable(complexCommitMsg),
		"pre-push.sample":        executable("#!/bin/sh\n"),
		prePush:                  {Data: []byte("#!/bin/sh\nexit 1\n"), Mode: 0o644},
		"post-checkout":          executable("#!/bin/sh\n# lefthook_version: 2\ncall_lefthook run post-checkout\n"),
		"prepare-commit-msg.old": executable("#!/bin/sh\n"),
		"notes.txt":              executable("not a hook"),
	}

	found := hooks.ExistingHooks(dir)

	// Git's own order, not the directory's: commit-msg after pre-commit. Not
	// a sample, not a file without the executable bit (git skips those), not
	// a hook lefthook already installed, not a leftover .old.
	names := make([]string, 0, len(found))
	for _, each := range found {
		names = append(names, each.Name)
	}

	if want := []string{preCommit, commitMsg}; !slices.Equal(names, want) {
		t.Errorf("ExistingHooks = %q, want %q", names, want)
	}

	if found[0].Script != simplePreCommit {
		t.Errorf("the script was not read whole: %q", found[0].Script)
	}
}

func TestHasConfigRecognizesEveryNameLefthookReads(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"lefthook.yml", ".lefthook.yml", "lefthook.yaml", ".lefthook.yaml", "lefthook.toml", ".lefthook.toml",
		"lefthook.json", ".lefthook.json", ".config/lefthook.yml",
	} {
		if !hooks.HasConfig(fstest.MapFS{name: {Data: []byte("x")}}) {
			t.Errorf("HasConfig did not see %s", name)
		}
	}

	if hooks.HasConfig(fstest.MapFS{"lefthook-local.yml": {Data: []byte("x")}, "README.md": {Data: []byte("x")}}) {
		t.Error("HasConfig saw a configuration in a repository without one")
	}
}

func TestStructuredTurnsPlainCommandsIntoOrderedJobs(t *testing.T) {
	t.Parallel()

	generated := hooks.Structured([]hooks.GitHook{
		{Name: preCommit, Script: simplePreCommit},
		{Name: commitMsg, Script: complexCommitMsg},
	})

	// Piped, so the first failure stops the rest as set -e did; numbered,
	// because lefthook runs piped commands in the order of their NAMES —
	// checked against lefthook 2.1.
	for _, want := range []string{
		"pre-commit:\n  piped: true\n  commands:\n",
		"    01-gofmt:\n      run: gofmt -l .\n",
		"    02-go-vet:\n      run: go vet ./...\n",
		// A hook that reads its argument or branches stays a script, which
		// lefthook hands git's arguments.
		"commit-msg:\n  scripts:\n    commit-msg:\n      runner: bash\n",
	} {
		if !strings.Contains(generated.Config, want) {
			t.Errorf("the configuration is missing\n%s\nin\n%s", want, generated.Config)
		}
	}

	want := []hooks.File{{Path: ".lefthook/commit-msg/commit-msg", Contents: complexCommitMsg, Executable: true}}
	if !slices.Equal(generated.Scripts, want) {
		t.Errorf("Scripts = %+v, want %+v", generated.Scripts, want)
	}
}

func TestACommandThatReadsAsAnotherTypeStaysAString(t *testing.T) {
	t.Parallel()

	// Found running a generated configuration through lefthook: a hook line of
	// just "false" was written as run: false, which YAML reads as a bool, and
	// lefthook ran the command "0". The same goes for true, null and numbers.
	generated := hooks.Structured([]hooks.GitHook{{Name: prePush, Script: "#!/bin/sh\nfalse\ntrue\n123\nyes\n"}})

	var config struct {
		PrePush struct {
			Commands map[string]struct {
				Run any `yaml:"run"`
			} `yaml:"commands"`
		} `yaml:"pre-push"` //nolint:tagliatelle // lefthook's key, named for git's hook
	}

	err := yaml.Unmarshal([]byte(generated.Config), &config)
	if err != nil {
		t.Fatalf("the configuration is not YAML: %v\n%s", err, generated.Config)
	}

	for name, command := range config.PrePush.Commands {
		if _, isString := command.Run.(string); !isString {
			t.Errorf("job %s runs %#v, a %T rather than a command\n%s", name, command.Run, command.Run, generated.Config)
		}
	}
}

func TestVerbatimKeepsEveryHookAsItsScript(t *testing.T) {
	t.Parallel()

	generated := hooks.Verbatim([]hooks.GitHook{{Name: preCommit, Script: simplePreCommit}})

	if !strings.Contains(generated.Config, "pre-commit:\n  scripts:\n    pre-commit:\n      runner: sh\n") {
		t.Errorf("the configuration does not run the script:\n%s", generated.Config)
	}

	if len(generated.Scripts) != 1 || generated.Scripts[0].Contents != simplePreCommit {
		t.Errorf("Scripts = %+v, want the hook copied whole", generated.Scripts)
	}
}

func TestAHookIsOnlyStructuredWhenEveryLineIsAPlainCommand(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a subshell":          "#!/bin/sh\necho $(date)\n",
		"backticks":           "#!/bin/sh\necho `date`\n",
		"an argument":         "#!/bin/sh\ncat \"$1\"\n",
		"all arguments":       "#!/bin/sh\nrun \"$@\"\n",
		"a heredoc":           "#!/bin/sh\ncat <<EOF\nx\nEOF\n",
		"a continued line":    "#!/bin/sh\ngo test \\\n  ./...\n",
		"a variable":          "#!/bin/sh\nFILES=a\nlint $FILES\n",
		"a loop":              "#!/bin/sh\nfor f in *; do echo $f; done\n",
		"a function":          "#!/bin/sh\ncheck() { true; }\ncheck\n",
		"a change of dir":     "#!/bin/sh\ncd sub\nmake\n",
		"another interpreter": "#!/usr/bin/env python3\nprint('hi')\n",
		"exit":                "#!/bin/sh\nexit 0\n",
	}

	for name, script := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			generated := hooks.Structured([]hooks.GitHook{{Name: preCommit, Script: script}})
			if strings.Contains(generated.Config, "commands:") || len(generated.Scripts) != 1 {
				t.Errorf("a hook with %s was turned into jobs:\n%s", name, generated.Config)
			}
		})
	}
}

func TestTheRunnerComesFromTheShebang(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"#!/bin/bash\n":               "bash",
		"#!/usr/bin/env python3\n":    "python3",
		"#!/usr/bin/env -S bash -e\n": "bash -e",
		"no shebang\n":                "sh",
		"#!\n":                        "sh",
	}

	for shebang, want := range cases {
		generated := hooks.Verbatim([]hooks.GitHook{{Name: prePush, Script: shebang + "exit 0\n"}})
		if !strings.Contains(generated.Config, "runner: "+want+"\n") {
			t.Errorf("a script starting %q got\n%s\nwant runner %q", shebang, generated.Config, want)
		}
	}
}

func TestAJobWithNoUsableNameIsStillNamed(t *testing.T) {
	t.Parallel()

	generated := hooks.Structured([]hooks.GitHook{{Name: preCommit, Script: "#!/bin/sh\n+++ check\n"}})
	if !strings.Contains(generated.Config, "01-job:") {
		t.Errorf("a command with no letters in its name got\n%s", generated.Config)
	}
}

func TestWriteReportsWhatItCouldNotCreate(t *testing.T) {
	t.Parallel()

	generated := hooks.Verbatim([]hooks.GitHook{{Name: preCommit, Script: "#!/bin/sh\nexit 1\n"}})

	err := hooks.Write(filepath.Join(t.TempDir(), "missing"), generated)
	if err == nil {
		t.Error("Write into a directory that does not exist returned nil")
	}

	// A file where the scripts directory belongs.
	blocked := t.TempDir()

	err = os.WriteFile(filepath.Join(blocked, ".lefthook"), []byte("x"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	err = hooks.Write(blocked, generated)
	if err == nil {
		t.Error("Write returned nil with a file in the way of .lefthook")
	}

	// An existing script is never replaced.
	taken := t.TempDir()

	err = os.MkdirAll(filepath.Join(taken, ".lefthook", preCommit), 0o750)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(taken, ".lefthook", preCommit, preCommit), []byte("mine"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	err = hooks.Write(taken, generated)
	if !errors.Is(err, fs.ErrExist) {
		t.Errorf("Write returned %v, want fs.ErrExist for an existing script", err)
	}
}

func TestWriteCreatesTheConfigurationAndScripts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	generated := hooks.Structured([]hooks.GitHook{
		{Name: preCommit, Script: simplePreCommit},
		{Name: commitMsg, Script: complexCommitMsg},
	})

	err := hooks.Write(dir, generated)
	if err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}

	config, err := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if err != nil || string(config) != generated.Config {
		t.Errorf("lefthook.yml = %q, %v", config, err)
	}

	script := filepath.Join(dir, ".lefthook", commitMsg, commitMsg)

	info, err := os.Stat(script)
	if err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("the script is missing or not executable: %v, %v", info, err)
	}

	// Never over an existing configuration.
	err = hooks.Write(dir, generated)
	if !errors.Is(err, fs.ErrExist) {
		t.Errorf("a second Write returned %v, want fs.ErrExist", err)
	}
}
