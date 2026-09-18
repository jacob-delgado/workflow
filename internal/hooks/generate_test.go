// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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

	// Arrange
	dir := fstest.MapFS{
		preCommit:                executable(simplePreCommit),
		commitMsg:                executable(complexCommitMsg),
		"pre-push.sample":        executable("#!/bin/sh\n"),
		prePush:                  {Data: []byte("#!/bin/sh\nexit 1\n"), Mode: 0o644},
		"post-checkout":          executable("#!/bin/sh\n# lefthook_version: 2\ncall_lefthook run post-checkout\n"),
		"prepare-commit-msg.old": executable("#!/bin/sh\n"),
		"notes.txt":              executable("not a hook"),
	}

	// Act
	found := hooks.ExistingHooks(dir, "linux")

	// Assert
	// Git's own order, not the directory's: commit-msg after pre-commit. Not
	// a sample, not a file without the executable bit (git skips those), not
	// a hook lefthook already installed, not a leftover .old.
	names := make([]string, 0, len(found))
	for _, each := range found {
		names = append(names, each.Name)
	}

	if want := []string{preCommit, commitMsg}; !slices.Equal(names, want) {
		t.Fatalf("ExistingHooks = %q, want %q", names, want)
	}

	if found[0].Script != simplePreCommit {
		t.Errorf("the script was not read whole: %q", found[0].Script)
	}
}

func TestExistingHooksNeedNoExecutableBitOnWindows(t *testing.T) {
	t.Parallel()

	// Arrange
	// Go reports no executable bit for any file on Windows, so requiring one
	// would find no hooks there; a plain, non-executable file must count.
	dir := fstest.MapFS{
		preCommit: {Data: []byte(simplePreCommit), Mode: 0o644},
	}

	// Act
	found := hooks.ExistingHooks(dir, "windows")

	// Assert
	if len(found) != 1 || found[0].Name != preCommit {
		t.Errorf("ExistingHooks on Windows = %+v, want the non-executable pre-commit found", found)
	}
}

func TestHasConfigRecognizesEveryNameLefthookReads(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"lefthook.yml", ".lefthook.yml", "lefthook.yaml", ".lefthook.yaml", "lefthook.toml", ".lefthook.toml",
		"lefthook.json", ".lefthook.json", ".config/lefthook.yml",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if !hooks.HasConfig(fstest.MapFS{name: {Data: []byte("x")}}) {
				t.Errorf("HasConfig did not see %s", name)
			}
		})
	}
}

func TestHasConfigSeesNoConfigurationWhereThereIsNone(t *testing.T) {
	t.Parallel()

	// Arrange
	repository := fstest.MapFS{"lefthook-local.yml": {Data: []byte("x")}, "README.md": {Data: []byte("x")}}

	// Act & Assert
	if hooks.HasConfig(repository) {
		t.Error("HasConfig saw a configuration in a repository without one")
	}
}

func TestStructuredTurnsPlainCommandsIntoOrderedJobs(t *testing.T) {
	t.Parallel()

	// Act
	generated := hooks.Structured([]hooks.GitHook{
		{Name: preCommit, Script: simplePreCommit},
		{Name: commitMsg, Script: complexCommitMsg},
	})

	// Assert
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

	want := []hooks.File{{Path: ".lefthook/commit-msg/commit-msg", Contents: complexCommitMsg}}
	if !slices.Equal(generated.Scripts, want) {
		t.Errorf("Scripts = %+v, want %+v", generated.Scripts, want)
	}
}

func TestACommandThatReadsAsAnotherTypeStaysAString(t *testing.T) {
	t.Parallel()

	// Arrange
	// Found running a generated configuration through lefthook: a hook line of
	// just "false" was written as run: false, which YAML reads as a bool, and
	// lefthook ran the command "0". The same goes for true, null and numbers.
	hook := hooks.GitHook{Name: prePush, Script: "#!/bin/sh\nset -e\nfalse\ntrue\n123\nyes\n"}

	// Act
	generated := hooks.Structured([]hooks.GitHook{hook})

	// Assert
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

	if len(config.PrePush.Commands) != 4 {
		t.Fatalf("pre-push has %d jobs, want one for each of the 4 commands\n%s",
			len(config.PrePush.Commands), generated.Config)
	}

	for name, command := range config.PrePush.Commands {
		if _, isString := command.Run.(string); !isString {
			t.Errorf("job %s runs %#v, a %T rather than a command\n%s", name, command.Run, command.Run, generated.Config)
		}
	}
}

func TestVerbatimKeepsEveryHookAsItsScript(t *testing.T) {
	t.Parallel()

	// Act
	generated := hooks.Verbatim([]hooks.GitHook{{Name: preCommit, Script: simplePreCommit}})

	// Assert
	if !strings.Contains(generated.Config, "pre-commit:\n  scripts:\n    pre-commit:\n      runner: sh\n") {
		t.Errorf("the configuration does not run the script:\n%s", generated.Config)
	}

	if len(generated.Scripts) != 1 || generated.Scripts[0].Contents != simplePreCommit {
		t.Errorf("Scripts = %+v, want the hook copied whole", generated.Scripts)
	}
}

func TestAHookIsOnlyStructuredWhenEveryLineIsAPlainCommand(t *testing.T) {
	t.Parallel()

	// Each carries set -e, so the reason it is kept whole is the shape on the
	// line rather than the missing errexit that the last cases test on their own.
	cases := map[string]string{
		"a subshell":            "#!/bin/sh\nset -e\necho $(date)\n",
		"backticks":             "#!/bin/sh\nset -e\necho `date`\n",
		"an argument":           "#!/bin/sh\nset -e\ncat \"$1\"\n",
		"all arguments":         "#!/bin/sh\nset -e\nrun \"$@\"\n",
		"a later argument":      "#!/bin/sh\nset -e\ndeploy $3\n",
		"the script name":       "#!/bin/sh\nset -e\necho $0\n",
		"the last status":       "#!/bin/sh\nset -e\nreport $?\n",
		"a heredoc":             "#!/bin/sh\nset -e\ncat <<EOF\nx\nEOF\n",
		"a continued line":      "#!/bin/sh\nset -e\ngo test \\\n  ./...\n",
		"a trailing and":        "#!/bin/sh\nset -e\nmake lint &&\nmake test\n",
		"a trailing or":         "#!/bin/sh\nset -e\nmake lint ||\nmake report\n",
		"a trailing pipe":       "#!/bin/sh\nset -e\nmake lint |\ntee log\n",
		"a pipeline":            "#!/bin/sh\nset -e\nmake lint | tee log\n",
		"a background job":      "#!/bin/sh\nset -e\nserve &\nwait\n",
		"process substitution":  "#!/bin/sh\nset -e\ndiff <(sort a) <(sort b)\n",
		"a variable":            "#!/bin/sh\nset -e\nFILES=a\nlint $FILES\n",
		"a loop":                "#!/bin/sh\nset -e\nfor f in *; do echo $f; done\n",
		"a function":            "#!/bin/sh\nset -e\ncheck() { true; }\ncheck\n",
		"a change of dir":       "#!/bin/sh\nset -e\ncd sub\nmake\n",
		"a subshell over lines": "#!/bin/sh\nset -e\n(\ncd sub\nmake\n)\n",
		"a directory stack":     "#!/bin/sh\nset -e\npushd sub\n",
		"a umask":               "#!/bin/sh\nset -e\numask 022\n",
		"an eval":               "#!/bin/sh\nset -e\neval make\n",
		"another interpreter":   "#!/usr/bin/env python3\nset -e\nprint('hi')\n",
		"a bash script":         "#!/bin/bash\nset -e\nmake\n",
		"a zsh script":          "#!/bin/zsh\nset -e\nmake\n",
		"pipefail it cannot keep": "#!/bin/sh\nset -e\nset -o pipefail\n" +
			"make lint | tee log\n",
		"exit": "#!/bin/sh\nset -e\nexit 0\n",
		"no errexit at all": "#!/bin/sh\n# no set -e, so a failure must not stop the rest\n" +
			"gofmt -l .\ngo vet ./...\n",
	}

	for name, script := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			generated := hooks.Structured([]hooks.GitHook{{Name: preCommit, Script: script}})

			// Assert
			if strings.Contains(generated.Config, "commands:") || len(generated.Scripts) != 1 {
				t.Errorf("a hook with %s was turned into jobs:\n%s", name, generated.Config)
			}
		})
	}
}

func TestTheRunnerComesFromTheShebang(t *testing.T) {
	t.Parallel()

	const bash = "bash"

	cases := map[string]string{
		"#!/bin/bash\n":               bash,
		"#!/usr/bin/env python3\n":    "python3",
		"#!/usr/bin/env -S bash -e\n": "bash -e",
		"no shebang\n":                "sh",
		"#!\n":                        "sh",
		// env with no interpreter must not read past the end of the line.
		"#!/usr/bin/env\n": "sh",
		// -u NAME unsets a variable; NAME is not the interpreter.
		"#!/usr/bin/env -u FOO bash\n": bash,
		// VAR=value is an assignment env consumes; it is not the interpreter.
		"#!/usr/bin/env VAR=1 bash\n": bash,
	}

	for shebang, want := range cases {
		t.Run(strconv.Quote(shebang), func(t *testing.T) {
			t.Parallel()

			// Act
			generated := hooks.Verbatim([]hooks.GitHook{{Name: prePush, Script: shebang + "exit 0\n"}})

			// Assert
			if !strings.Contains(generated.Config, "runner: "+want+"\n") {
				t.Errorf("a script starting %q got\n%s\nwant runner %q", shebang, generated.Config, want)
			}
		})
	}
}

func TestAJobWithNoUsableNameIsStillNamed(t *testing.T) {
	t.Parallel()

	// Act
	generated := hooks.Structured([]hooks.GitHook{{Name: preCommit, Script: "#!/bin/sh\nset -e\n+++ check\n"}})

	// Assert
	if !strings.Contains(generated.Config, "01-job:") {
		t.Errorf("a command with no letters in its name got\n%s", generated.Config)
	}
}

// failingHook is a hook written verbatim, for the tests of where it is written.
func failingHook() hooks.Generated {
	return hooks.Verbatim([]hooks.GitHook{{Name: preCommit, Script: "#!/bin/sh\nexit 1\n"}})
}

func TestWriteIntoADirectoryThatDoesNotExistSaysWhatItCouldNotCreate(t *testing.T) {
	t.Parallel()

	// Arrange
	missing := filepath.Join(t.TempDir(), "missing")

	// Act
	err := hooks.Write(missing, failingHook())

	// Assert
	wantNamed := "creating " + filepath.Join(missing, "lefthook.yml")
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), wantNamed) {
		t.Errorf("Write = %v, want it to name the lefthook.yml it could not create", err)
	}
}

func TestWriteWithAFileInTheWayOfTheScriptsSaysWhatItCouldNotCreate(t *testing.T) {
	t.Parallel()

	// Arrange
	blocked := t.TempDir()

	err := os.WriteFile(filepath.Join(blocked, ".lefthook"), []byte("x"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	err = hooks.Write(blocked, failingHook())

	// Assert
	if err == nil || !strings.Contains(err.Error(), "creating .lefthook/pre-commit") {
		t.Errorf("Write = %v, want it to name the scripts directory it could not create", err)
	}
}

func TestWriteNeverReplacesAnExistingScript(t *testing.T) {
	t.Parallel()

	// Arrange
	taken := t.TempDir()

	err := os.MkdirAll(filepath.Join(taken, ".lefthook", preCommit), 0o750)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(taken, ".lefthook", preCommit, preCommit), []byte("mine"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	err = hooks.Write(taken, failingHook())

	// Assert
	if !errors.Is(err, fs.ErrExist) {
		t.Errorf("Write returned %v, want fs.ErrExist for an existing script", err)
	}

	mine, readErr := os.ReadFile(filepath.Join(taken, ".lefthook", preCommit, preCommit))
	if readErr != nil || string(mine) != "mine" {
		t.Errorf("the existing script is now %q, %v", mine, readErr)
	}
}

func TestAFailedWriteLeavesNoConfigurationBehind(t *testing.T) {
	t.Parallel()

	// Arrange
	// A leftover script from an earlier failed run blocks the write.
	taken := t.TempDir()

	err := os.MkdirAll(filepath.Join(taken, ".lefthook", preCommit), 0o750)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(taken, ".lefthook", preCommit, preCommit), []byte("mine"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	err = hooks.Write(taken, failingHook())

	// Assert
	// The write failed, and left no lefthook.yml — so the offer is not blocked
	// from being tried again by a configuration that names a script it never
	// managed to write.
	if err == nil {
		t.Fatal("Write succeeded, want it to fail on the leftover script")
	}

	_, statErr := os.Stat(filepath.Join(taken, "lefthook.yml"))
	if !os.IsNotExist(statErr) {
		t.Errorf("a failed write left a lefthook.yml behind (stat: %v)", statErr)
	}
}

// generatedForBoth is a configuration with a job hook and a script hook.
func generatedForBoth() hooks.Generated {
	return hooks.Structured([]hooks.GitHook{
		{Name: preCommit, Script: simplePreCommit},
		{Name: commitMsg, Script: complexCommitMsg},
	})
}

func TestWriteCreatesTheConfigurationAndScripts(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	generated := generatedForBoth()

	// Act
	err := hooks.Write(dir, generated)
	if err != nil {
		t.Fatalf("Write returned %v, want nil", err)
	}

	// Assert
	config, err := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if err != nil || string(config) != generated.Config {
		t.Errorf("lefthook.yml = %q, %v", config, err)
	}

	script := filepath.Join(dir, ".lefthook", commitMsg, commitMsg)

	info, err := os.Stat(script)
	if err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("the script is missing or not executable: %v, %v", info, err)
	}
}

func TestWriteNeverReplacesAConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	err := hooks.Write(dir, generatedForBoth())
	if err != nil {
		t.Fatalf("the first Write returned %v, want nil", err)
	}

	// Act
	err = hooks.Write(dir, generatedForBoth())

	// Assert
	if !errors.Is(err, fs.ErrExist) {
		t.Errorf("a second Write returned %v, want fs.ErrExist", err)
	}
}
