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

// gitProgram is the program every command builder runs, commitVerb its
// subcommand the commit builders share.
const (
	gitProgram = "git"
	commitVerb = "commit"
)

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
			err := gitrepo.At(run, workDir).CreateBranch(t.Context(), tt.name, tt.start)

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
	push := gitrepo.PushCommand(workDir, "upstream", "fix/PROJ-1-x")

	// Assert
	if push.Dir != workDir || push.Name != gitProgram ||
		!slices.Equal(push.Args, []string{"push", "--set-upstream", "upstream", "fix/PROJ-1-x"}) {
		t.Errorf("PushCommand = %+v", push)
	}

	// Nobody can answer a credential prompt from inside the interface, so git
	// must fail rather than wait for one.
	if !slices.Contains(push.Env, "GIT_TERMINAL_PROMPT=0") {
		t.Errorf("PushCommand environment = %q, want prompts turned off", push.Env)
	}
}

func TestPushRemoteHonorsPushDefaultAndFallsBackToOrigin(t *testing.T) {
	t.Parallel()

	// Arrange
	const query = "git -C " + workDir + " config --get remote.pushDefault"

	set := fakeRunner(t, map[string]reply{query: {out: []byte("upstream\n")}})
	// git exits non-zero when the key is unset.
	unset := fakeRunner(t, map[string]reply{query: {err: errUnexpectedCommand}})

	// Act
	configured := gitrepo.At(set, workDir).PushRemote(t.Context())
	fallback := gitrepo.At(unset, workDir).PushRemote(t.Context())

	// Assert
	if configured != "upstream" {
		t.Errorf("PushRemote with remote.pushDefault set = %q, want upstream", configured)
	}

	if fallback != "origin" {
		t.Errorf("PushRemote with no default = %q, want origin", fallback)
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

func TestPullCommandFastForwardsWithoutPrompts(t *testing.T) {
	t.Parallel()

	// Act
	pull := gitrepo.PullCommand(workDir)

	// Assert
	if pull.Dir != workDir || pull.Name != gitProgram ||
		!slices.Equal(pull.Args, []string{"pull", "--ff-only"}) {
		t.Errorf("PullCommand = %+v", pull)
	}

	if !slices.Contains(pull.Env, "GIT_TERMINAL_PROMPT=0") {
		t.Errorf("PullCommand environment = %q, want prompts turned off", pull.Env)
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

func TestCheckIgnoredReportsWhetherGitIgnoresThePath(t *testing.T) {
	t.Parallel()

	const (
		probe   = "git -C /work rev-parse --show-toplevel"
		inquire = "git -C /work check-ignore /work/.workflow.json"
	)

	cases := map[string]struct {
		replies map[string]reply
		want    bool
		wantErr error
	}{
		"ignored":     {replies: map[string]reply{probe: {}, inquire: {}}, want: true},
		"not ignored": {replies: map[string]reply{probe: {}, inquire: {err: errNotIgnored}}, want: false},
		"outside a repository": {
			replies: map[string]reply{probe: {err: errNotARepository}}, wantErr: gitrepo.ErrNotARepository,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run := fakeRunner(t, tt.replies)

			// Act
			ignored, err := gitrepo.At(run, workDir).CheckIgnored(t.Context(), "/work/.workflow.json")

			// Assert
			if ignored != tt.want || !errors.Is(err, tt.wantErr) {
				t.Errorf("CheckIgnored = %t, %v; want %t, %v", ignored, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestCommitCommandRunsInTheRepository(t *testing.T) {
	t.Parallel()

	// Act
	commit := gitrepo.CommitCommand(workDir, "/tmp/message.txt")

	// Assert
	want := proc.Command{
		Dir: workDir, Name: gitProgram,
		Args: []string{commitVerb, "--file", "/tmp/message.txt"}, Env: nil,
	}
	if commit.Dir != want.Dir || commit.Name != want.Name || !slices.Equal(commit.Args, want.Args) {
		t.Errorf("CommitCommand = %+v, want %+v", commit, want)
	}
}

func TestAmendCommandFoldsTheIndexIntoTheLastCommit(t *testing.T) {
	t.Parallel()

	// Act
	amend := gitrepo.AmendCommand(workDir)

	// Assert
	// Env is nil, like CommitCommand, so the repository's hooks run.
	want := []string{commitVerb, "--amend", "--no-edit"}
	if amend.Dir != workDir || amend.Name != gitProgram || !slices.Equal(amend.Args, want) || amend.Env != nil {
		t.Errorf("AmendCommand = %+v, want args %q with nil env", amend, want)
	}
}

func TestFixupCommandRecordsAFixupOfTheChosenCommit(t *testing.T) {
	t.Parallel()

	// Act
	fixup := gitrepo.FixupCommand(workDir, "1a2b3c4")

	// Assert
	want := []string{commitVerb, "--fixup=1a2b3c4"}
	if fixup.Dir != workDir || fixup.Name != gitProgram || !slices.Equal(fixup.Args, want) || fixup.Env != nil {
		t.Errorf("FixupCommand = %+v, want args %q with nil env", fixup, want)
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
			dir, err := gitrepo.At(fakeRunner(t, replies), workDir).HooksDir(t.Context())

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
	_, err := gitrepo.At(fakeRunner(t, replies), workDir).HooksDir(t.Context())

	// Assert
	if !errors.Is(err, errNotARepository) {
		t.Errorf("HooksDir returned %v, want git's error", err)
	}
}
