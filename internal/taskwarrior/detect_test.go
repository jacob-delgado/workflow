// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

func TestCandidatesListsEveryTaskOnPathInOrder(t *testing.T) {
	t.Parallel()

	// file is one program in one of the three directories on the PATH list.
	type file struct {
		dir  int
		name string
		kind int
		mode os.FileMode
	}

	tests := []struct {
		name  string
		goos  string
		files []file
		want  []file
	}{
		{
			name: "only an executable task counts",
			goos: linux,
			files: []file{
				{dir: 0, name: taskProgram, mode: 0o755},
				{dir: 1, name: taskProgram, mode: 0o644},
				{dir: 2, name: taskProgram, mode: 0o755},
			},
			want: []file{{dir: 0, name: taskProgram}, {dir: 2, name: taskProgram}},
		},
		{
			name: "on windows task.exe counts and the bit does not",
			goos: windows,
			files: []file{
				{dir: 0, name: "task.exe", mode: 0o644},
				{dir: 0, name: taskProgram, mode: 0o644},
				{dir: 1, name: taskProgram, mode: 0o644},
			},
			want: []file{{dir: 0, name: "task.exe"}, {dir: 1, name: taskProgram}},
		},
		{
			name:  "a link to an executable task counts",
			goos:  linux,
			files: []file{{dir: 1, name: taskProgram, kind: symlink}},
			want:  []file{{dir: 1, name: taskProgram}},
		},
		{
			name: "a directory named task does not",
			goos: linux,
			files: []file{
				{dir: 0, name: taskProgram, kind: directory, mode: 0o755},
				{dir: 2, name: taskProgram, mode: 0o755},
			},
			want: []file{{dir: 2, name: taskProgram}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dirs := []string{t.TempDir(), t.TempDir(), t.TempDir()}
			for _, program := range test.files {
				place(t, filepath.Join(dirs[program.dir], program.name), program.kind, program.mode)
			}

			separator := ":"
			if test.goos == windows {
				separator = ";"
			}

			pathList := strings.Join(append([]string{""}, dirs...), separator)

			var want []string
			for _, program := range test.want {
				want = append(want, filepath.Join(dirs[program.dir], program.name))
			}

			// Act
			got := taskwarrior.Candidates(pathList, test.goos)

			// Assert
			if !slices.Equal(got, want) {
				t.Errorf("Candidates = %q, want %q", got, want)
			}
		})
	}
}

func TestCandidatesLeavesOutARelativePathEntry(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	place(t, filepath.Join(dir, taskProgram), plainFile, 0o755)

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	relative, err := filepath.Rel(workDir, dir)
	if err != nil {
		t.Fatal(err)
	}

	pathList := relative + ":" + dir

	// Act
	got := taskwarrior.Candidates(pathList, linux)

	// Assert
	if want := []string{filepath.Join(dir, taskProgram)}; !slices.Equal(got, want) {
		t.Errorf("Candidates(%q) = %q, want only the absolute entry's %q", pathList, got, want)
	}
}

func TestCandidatesReadsAQuotedWindowsPathEntry(t *testing.T) {
	t.Parallel()

	// Each case writes a directory into a Windows PATH list in double quotes,
	// as Windows allows so a ; in its name does not end the entry, beside a
	// plain one; quotedLast puts the quoted entry after the plain one.
	tests := map[string]struct {
		list       func(quoted, plain string) string
		quotedLast bool
	}{
		"a quoted entry": {
			list: func(quoted, plain string) string { return `"` + quoted + `";` + plain },
		},
		"a partly quoted entry": {
			list: func(quoted, plain string) string {
				parent, name := filepath.Split(quoted)

				return parent + `"` + name + `";` + plain
			},
		},
		"a quoted entry last": {
			list:       func(quoted, plain string) string { return plain + `;"` + quoted + `"` },
			quotedLast: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			quoted := filepath.Join(t.TempDir(), "Task;warrior")
			place(t, quoted, directory, 0o755)

			plain := t.TempDir()
			for _, dir := range []string{quoted, plain} {
				place(t, filepath.Join(dir, "task.exe"), plainFile, 0o644)
			}

			pathList := test.list(quoted, plain)

			want := []string{filepath.Join(quoted, "task.exe"), filepath.Join(plain, "task.exe")}
			if test.quotedLast {
				slices.Reverse(want)
			}

			// Act
			got := taskwarrior.Candidates(pathList, windows)

			// Assert
			if !slices.Equal(got, want) {
				t.Errorf("Candidates(%q) = %q, want %q", pathList, got, want)
			}
		})
	}
}

func TestCandidatesTakesAQuoteAsPartOfANameOutsideWindows(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := filepath.Join(t.TempDir(), `say"hi`)
	place(t, dir, directory, 0o755)
	place(t, filepath.Join(dir, taskProgram), plainFile, 0o755)

	// Act
	got := taskwarrior.Candidates(dir, linux)

	// Assert
	if want := []string{filepath.Join(dir, taskProgram)}; !slices.Equal(got, want) {
		t.Errorf("Candidates(%q) = %q, want %q", dir, got, want)
	}
}

func TestDetectKeepsTheFirstCandidateThatIsTaskwarrior(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeTask{replies: map[string]reply{
		goTask + " _version": {err: exited(t, 200, goTaskAnswer)},
		"tw _version":        {stdout: versionAnswer},
		showWord:             {stdout: "data.location=/d\n"},
	}}

	// Act
	install, err := taskwarrior.Detect(t.Context(), "", []string{goTask, "tw"}, fake.run)
	// Assert
	if err != nil {
		t.Fatalf("Detect returned %v", err)
	}

	if install.Program != "tw" || install.Version != taskwarrior.MinimumVersion {
		t.Errorf("Detect = %+v, want tw at 3.5.0", install)
	}

	askedVersion := []string{"rc.hooks=off", versionWord}
	checkRuns(t, fake.calls, []call{
		{program: goTask, args: askedVersion},
		{program: "tw", args: askedVersion},
		{program: "tw", args: append(baseReadOverrides(), showWord)},
	})
}

func TestDetectTriesOnlyTheConfiguredProgram(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeTask{replies: map[string]reply{
		versionWord: {stdout: versionAnswer},
		showWord:    {stdout: ""},
	}}

	// Act
	install, err := taskwarrior.Detect(t.Context(), "/x/task", []string{goTask, "tw"}, fake.run)

	// Assert
	if err != nil || install.Program != "/x/task" {
		t.Fatalf("Detect = %+v, %v; want /x/task", install, err)
	}

	for _, run := range fake.calls {
		if run.program != "/x/task" {
			t.Errorf("Detect ran %s, want only /x/task", run.program)
		}
	}
}

func TestDetectRefusesAnOlderTaskwarrior(t *testing.T) {
	t.Parallel()

	const older = "/usr/bin/task"

	tests := []struct {
		name       string
		candidates []string
		replies    map[string]reply
		version    string
	}{
		{
			name: "one older Taskwarrior", candidates: []string{older},
			replies: map[string]reply{versionWord: {stdout: "3.4.2\n"}}, version: "3.4.2",
		},
		{
			name:       "the first older one after go-task",
			candidates: []string{goTask, older, "/old/task"},
			replies: map[string]reply{
				goTask + " _version": {err: exited(t, 200, goTaskAnswer)},
				older + " _version":  {stdout: "3.4.2\n"},
				"/old/task _version": {stdout: "3.4.0\n"},
			},
			version: "3.4.2",
		},
		{
			name: "a patch release past nine", candidates: []string{older},
			replies: map[string]reply{versionWord: {stdout: "3.4.10\n"}}, version: "3.4.10",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: test.replies}

			// Act
			_, err := taskwarrior.Detect(t.Context(), "", test.candidates, fake.run)

			// Assert
			if !errors.Is(err, taskwarrior.ErrTooOld) || errors.Is(err, taskwarrior.ErrNotTaskwarrior) {
				t.Fatalf("Detect returned %v, want ErrTooOld and not ErrNotTaskwarrior", err)
			}

			for _, fact := range []string{test.version, older, taskwarrior.MinimumVersion} {
				if !strings.Contains(err.Error(), fact) {
					t.Errorf("Detect's error %q does not name %s", err, fact)
				}
			}
		})
	}
}

func TestDetectSaysWhenNothingOnPathIsTaskwarrior(t *testing.T) {
	t.Parallel()

	candidates := []string{"/a/task", "/b/task", "/c/task", "/d/task"}
	nothing := map[string]reply{
		"/a/task _version": {err: exited(t, 200, goTaskAnswer)},
		"/b/task _version": {stdout: "usage: task [flags] [tasks...]\n"},
		"/c/task _version": {stdout: "99999999999999999999.0.0\n"},
		"/d/task _version": {err: fmt.Errorf("%w: /d/task", proc.ErrNotFound)},
	}
	found := map[string]reply{versionWord: {stdout: versionAnswer}, showWord: {}}
	neverRun := map[string]reply{versionWord: {stdout: versionAnswer}, showWord: {err: exited(t, 2, noRCFile)}}

	tests := []struct {
		name          string
		replies       map[string]reply
		canceled      bool   // before Detect runs
		cancelAfter   string // the command word whose answer cancels the context
		want, notWant error
		named         string
		runs, live    int // at most runs, and exactly live of them on a live context
	}{
		{
			name: "each answers as another program or is not there", replies: nothing, want: taskwarrior.ErrNotTaskwarrior,
			named: strings.Join(candidates, ", "), runs: len(candidates), live: len(candidates),
		},
		{
			name: "a canceled context stops the walk", replies: nothing, canceled: true,
			want: context.Canceled, notWant: taskwarrior.ErrNotTaskwarrior, runs: 1,
		},
		{name: "canceled before Taskwarrior answers", replies: found, canceled: true, want: context.Canceled, runs: 1},
		{
			name: "a cancel once _version answers reaches _show", replies: found, cancelAfter: versionWord,
			want: context.Canceled, runs: 2, live: 1,
		},
		{
			name: "a cancel as _show fails is the context's", replies: neverRun, cancelAfter: showWord,
			want: context.Canceled, notWant: taskwarrior.ErrNotConfigured, runs: 2, live: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: test.replies}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			if test.canceled {
				cancel()
			}

			// Act
			_, err := taskwarrior.Detect(ctx, "", candidates, fake.cancelingAfter(test.cancelAfter, cancel))

			// Assert
			if !errors.Is(err, test.want) || errors.Is(err, test.notWant) {
				t.Fatalf("Detect returned %v, want %v and not %v", err, test.want, test.notWant)
			}

			if !strings.Contains(err.Error(), test.named) {
				t.Errorf("Detect's error %q does not list the paths it tried", err)
			}

			if len(fake.calls) > test.runs || liveRuns(fake.calls) != test.live {
				t.Errorf("Detect ran %+v, want at most %d runs, %d of them on a live context", fake.calls, test.runs, test.live)
			}
		})
	}
}

func TestDetectReportsATaskwarriorNeverRun(t *testing.T) {
	t.Parallel()

	const neverRun = "/opt/homebrew/bin/task"

	tests := []struct {
		name       string
		candidates []string
		replies    map[string]reply
		named      string
		ran        []string // each run's program and command word
	}{
		{
			name:       "its _version cannot proceed",
			candidates: []string{neverRun, "/usr/local/bin/task"},
			replies: map[string]reply{
				neverRun + " _version":         {err: exited(t, 2, noRCFile)},
				"/usr/local/bin/task _version": {stdout: versionAnswer},
				showWord:                       {stdout: ""},
			},
			named: neverRun, ran: []string{neverRun + " " + versionWord},
		},
		{
			name: "its _show cannot proceed", candidates: []string{neverRun},
			replies: map[string]reply{versionWord: {stdout: versionAnswer}, showWord: {err: exited(t, 2, noRCFile)}},
			named:   neverRun, ran: []string{neverRun + " " + versionWord, neverRun + " " + showWord},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: test.replies}

			// Act
			_, err := taskwarrior.Detect(t.Context(), "", test.candidates, fake.run)

			// Assert
			var never taskwarrior.NeverRunError
			if !errors.Is(err, taskwarrior.ErrNotConfigured) || !errors.As(err, &never) || never.Program != test.named {
				t.Errorf("Detect returned %v, want ErrNotConfigured naming the program %q", err, test.named)
			}

			if err != nil && !strings.HasSuffix(err.Error(), ": "+test.named) {
				t.Errorf("Detect's error reads %q, want it to end naming %q", err, test.named)
			}

			ran := make([]string, 0, len(fake.calls))
			for _, run := range fake.calls {
				ran = append(ran, run.program+" "+run.args[len(run.args)-1])
			}

			if !slices.Equal(ran, test.ran) {
				t.Errorf("Detect ran %q, want %q", ran, test.ran)
			}
		})
	}
}

func TestDetectReadsTheDataDirSyncAndUDAFromShow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		show string
		want taskwarrior.Install
	}{
		{name: "the data directory", show: "data.location=/d\n", want: taskwarrior.Install{DataDir: "/d"}},
		{name: "a sync server", show: "sync.server.url=https://s\n", want: taskwarrior.Install{SyncConfigured: true}},
		{name: "a sync origin", show: "sync.server.origin=https://s\n", want: taskwarrior.Install{SyncConfigured: true}},
		{name: "a local sync", show: "sync.local.server_dir=/l\n", want: taskwarrior.Install{SyncConfigured: true}},
		{name: "a gcp sync", show: "sync.gcp.bucket=b\n", want: taskwarrior.Install{SyncConfigured: true}},
		{name: "an aws sync", show: "sync.aws.bucket=b\n", want: taskwarrior.Install{SyncConfigured: true}},
		{name: "a git sync", show: "sync.git.local_path=/g\n", want: taskwarrior.Install{SyncConfigured: true}},
		{name: "the link UDA", show: "uda.jiraid.type=string\n", want: taskwarrior.Install{LinkUDADefined: true}},
		{name: "none of them", show: "color=on\nsync.server.url=\nverbose\n", want: taskwarrior.Install{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: map[string]reply{
				versionWord: {stdout: versionAnswer},
				showWord:    {stdout: test.show},
			}}
			want := test.want
			want.Program, want.Version = taskProgram, taskwarrior.MinimumVersion

			// Act
			install, err := taskwarrior.Detect(t.Context(), taskProgram, nil, fake.run)

			// Assert
			if err != nil || install != want {
				t.Errorf("Detect = %+v, %v; want %+v", install, err, want)
			}

			show := fake.calls[len(fake.calls)-1].args
			if show[len(show)-1] != showWord || slices.ContainsFunc(show, func(arg string) bool {
				return strings.HasPrefix(arg, "rc.uda.")
			}) {
				t.Errorf("Detect's last run was task %q, want _show with no UDA of its own", show)
			}
		})
	}
}

func TestDetectKeepsNoSecretFromShow(t *testing.T) {
	t.Parallel()

	const (
		secret = "hunter2"
		show   = "data.location=/d\nsync.encryption_secret=" + secret + "\nsync.server.url=https://s\n"
	)

	tests := []struct {
		name    string
		show    reply
		dataDir string
		failed  bool
	}{
		{name: "a _show that answers", show: reply{stdout: show}, dataDir: "/d"},
		{name: "a _show that fails", show: reply{stdout: show, err: exited(t, 2, "Configuration error.")}, failed: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: map[string]reply{versionWord: {stdout: versionAnswer}, showWord: test.show}}

			// Act
			install, err := taskwarrior.Detect(t.Context(), taskProgram, nil, fake.run)

			// Assert
			if install.DataDir != test.dataDir || (err != nil) != test.failed {
				t.Fatalf("Detect = %+v, %v; want data directory %q, failed %t", install, err, test.dataDir, test.failed)
			}

			if seen := fmt.Sprintf("%+v %v", install, err); strings.Contains(seen, secret) {
				t.Errorf("Detect kept the sync secret: %s", seen)
			}
		})
	}
}

func TestDetectWithNoCandidatesIsNotInstalled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		program    string
		candidates []string
		named      string
		runs       int
	}{
		{name: "no program and no candidates"},
		{name: "a configured program that does not exist", program: "/nowhere/task", named: "/nowhere/task", runs: 1},
		{name: "candidates all gone", candidates: []string{"/a/task", "/b/task"}, named: "/a/task, /b/task", runs: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: map[string]reply{versionWord: {err: fmt.Errorf("%w: task", proc.ErrNotFound)}}}

			// Act
			_, err := taskwarrior.Detect(t.Context(), test.program, test.candidates, fake.run)

			// Assert
			if !errors.Is(err, taskwarrior.ErrNotInstalled) || errors.Is(err, taskwarrior.ErrNotTaskwarrior) {
				t.Fatalf("Detect returned %v, want ErrNotInstalled and not ErrNotTaskwarrior", err)
			}

			if !strings.Contains(err.Error(), test.named) {
				t.Errorf("Detect's error %q does not name %s", err, test.named)
			}

			if len(fake.calls) != test.runs {
				t.Errorf("Detect ran %+v, want %d runs", fake.calls, test.runs)
			}
		})
	}
}

func TestDetectStripsTheCommitFromTheVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		printed string
		want    string
	}{
		{printed: taskwarrior.MinimumVersion + " (abc123)\n", want: taskwarrior.MinimumVersion},
		{printed: "3.10.0 (abc123)\n", want: "3.10.0"},
		{printed: "10.0.0\n", want: "10.0.0"},
	}

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: map[string]reply{versionWord: {stdout: test.printed}, showWord: {stdout: ""}}}

			// Act
			install, err := taskwarrior.Detect(t.Context(), taskProgram, nil, fake.run)

			// Assert
			if err != nil || install.Version != test.want {
				t.Errorf("Detect = %+v, %v; want version %s", install, err, test.want)
			}
		})
	}
}
