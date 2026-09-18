// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// gitProgram is the program every command builder runs.
const gitProgram = "git"

func TestCreateBranchStartsFromTheBaseWithoutTrackingIt(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name, start string
		command     string
		answer      reply
		wantErr     error
	}{
		// Without --no-track, a branch started from origin/main would take
		// origin/main as its upstream, and read as behind or ahead of main.
		"from a base": {
			name: "fix/PROJ-1-x", start: "origin/main",
			command: "git -C /work switch --create fix/PROJ-1-x --no-track origin/main",
		},
		"from HEAD": {name: "feat/PROJ-2-y", start: "", command: "git -C /work switch --create feat/PROJ-2-y"},
		"refused by git": {
			name: "taken", start: localMain,
			command: "git -C /work switch --create taken --no-track main",
			answer:  reply{err: errIndexLocked}, wantErr: errIndexLocked,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run, ran := recordingRunner(t, map[string]reply{tt.command: tt.answer})

			// Act
			err := gitrepo.CreateBranch(t.Context(), run, workDir, tt.name, tt.start)

			// Assert
			if !errors.Is(err, tt.wantErr) || !slices.Equal(*ran, []string{tt.command}) {
				t.Errorf("CreateBranch ran %q and returned %v, want %q and %v", *ran, err, tt.command, tt.wantErr)
			}
		})
	}
}

func TestPushCommandRunsInTheRepositoryWithoutPrompts(t *testing.T) {
	t.Parallel()

	// Act
	push := gitrepo.PushCommand(workDir, "fix/PROJ-1-x")

	// Assert
	if push.Dir != workDir || push.Name != gitProgram ||
		!slices.Equal(push.Args, []string{"push", "--set-upstream", "origin", "fix/PROJ-1-x"}) {
		t.Errorf("PushCommand = %+v", push)
	}

	// Nobody can answer a credential prompt from inside the interface, so git
	// must fail rather than wait for one.
	if !slices.Contains(push.Env, "GIT_TERMINAL_PROMPT=0") {
		t.Errorf("PushCommand environment = %q, want prompts turned off", push.Env)
	}
}

func TestFetchCommandUpdatesOriginWithoutPrompts(t *testing.T) {
	t.Parallel()

	// Act
	fetch := gitrepo.FetchCommand(workDir)

	// Assert
	if fetch.Dir != workDir || fetch.Name != gitProgram ||
		!slices.Equal(fetch.Args, []string{"fetch", "origin"}) {
		t.Errorf("FetchCommand = %+v", fetch)
	}

	if !slices.Contains(fetch.Env, "GIT_TERMINAL_PROMPT=0") {
		t.Errorf("FetchCommand environment = %q, want prompts turned off", fetch.Env)
	}
}

func TestRebaseCommandReplaysOntoTheBaseWithoutPrompts(t *testing.T) {
	t.Parallel()

	// Act
	rebase := gitrepo.RebaseCommand(workDir, "origin/main")

	// Assert
	if rebase.Dir != workDir || rebase.Name != gitProgram ||
		!slices.Equal(rebase.Args, []string{"rebase", "origin/main"}) {
		t.Errorf("RebaseCommand = %+v", rebase)
	}

	if !slices.Contains(rebase.Env, "GIT_TERMINAL_PROMPT=0") {
		t.Errorf("RebaseCommand environment = %q, want prompts turned off", rebase.Env)
	}
}

func TestCommitCommandRunsInTheRepository(t *testing.T) {
	t.Parallel()

	// Act
	commit := gitrepo.CommitCommand(workDir, "/tmp/message.txt")

	// Assert
	want := proc.Command{Dir: workDir, Name: gitProgram, Args: []string{"commit", "--file", "/tmp/message.txt"}, Env: nil}
	if commit.Dir != want.Dir || commit.Name != want.Name || !slices.Equal(commit.Args, want.Args) {
		t.Errorf("CommitCommand = %+v, want %+v", commit, want)
	}
}

func TestHooksDirIsWhereGitLooksForHooks(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		answer string
		want   string
	}{
		"relative to the repository":  {answer: ".git/hooks\n", want: "/work/.git/hooks"},
		"core.hooksPath set absolute": {answer: "/etc/githooks\n", want: "/etc/githooks"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			replies := map[string]reply{readHooksDir: {out: []byte(tt.answer)}}

			// Act
			dir, err := gitrepo.HooksDir(t.Context(), fakeRunner(t, replies), workDir)

			// Assert
			if err != nil || dir != tt.want {
				t.Errorf("HooksDir = %q, %v, want %q", dir, err, tt.want)
			}
		})
	}
}

func TestHooksDirReportsGitsFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{readHooksDir: {err: errNotARepository}}

	// Act
	_, err := gitrepo.HooksDir(t.Context(), fakeRunner(t, replies), workDir)

	// Assert
	if !errors.Is(err, errNotARepository) {
		t.Errorf("HooksDir returned %v, want git's error", err)
	}
}
