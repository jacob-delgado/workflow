// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// upstreamName is the remote a fork's clone names for the repository it was
// forked from.
const upstreamName = "upstream"

// pushedBranch is the branch the push remote cases ask about, and
// readBranchPushRemote the key git keeps its own push remote under.
const (
	pushedBranch         = "feat/token"
	readBranchPushRemote = "git -C /work config --get branch." + pushedBranch + ".pushRemote"
	readBranchRemote     = "git -C /work config --get branch." + pushedBranch + ".remote"
)

func TestPushRemoteReadsTheBranchKeyBeforeTheDefault(t *testing.T) {
	t.Parallel()

	none := reply{err: errNoRef}

	cases := map[string]struct {
		own, pushDefault, tracked reply
		want                      string
	}{
		"the branch's own push remote": {
			own: reply{out: []byte(forkRemote + "\n")}, pushDefault: reply{out: []byte("elsewhere\n")},
			tracked: none, want: forkRemote,
		},
		"the push default over the remote it tracks": {
			own: none, pushDefault: reply{out: []byte(forkRemote + "\n")},
			tracked: reply{out: []byte("elsewhere\n")}, want: forkRemote,
		},
		// git push -u fork feat records branch.feat.remote, and a plain push
		// goes there after.
		"only the remote it tracks": {
			own: none, pushDefault: none, tracked: reply{out: []byte(forkRemote + "\n")}, want: forkRemote,
		},
		"none of them": {own: none, pushDefault: none, tracked: none, want: gitrepo.DefaultRemote},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := gitrepo.At(fakeRunner(t, map[string]reply{
				readBranchPushRemote: tt.own,
				readPushDefault:      tt.pushDefault,
				readBranchRemote:     tt.tracked,
			}), workDir)

			// Act
			got := repo.PushRemote(t.Context(), pushedBranch)

			// Assert
			if got != tt.want {
				t.Errorf("PushRemote = %q, want %q", got, tt.want)
			}
		})
	}
}

// A detached HEAD has no branch to keep a push remote under, so only the
// repository's default is asked; fakeRunner fails any other command.
func TestADetachedHeadAsksOnlyForThePushDefault(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{readPushDefault: {out: []byte(forkRemote + "\n")}}), workDir)

	// Act
	got := repo.PushRemote(t.Context(), "")

	// Assert
	if got != forkRemote {
		t.Errorf("PushRemote of a detached HEAD = %q, want %q", got, forkRemote)
	}
}

// readRemotes is the one read that names the repository's remotes and what gh
// marked on them.
const readRemotes = `git -C /work config -z --get-regexp ^remote\.`

// remoteConfig is git config -z's answer for records of key and value.
func remoteConfig(records ...string) reply {
	var out strings.Builder

	for index := 0; index+1 < len(records); index += 2 {
		out.WriteString(records[index] + "\n" + records[index+1] + "\x00")
	}

	return reply{out: []byte(out.String())}
}

func TestBaseRemote(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		config reply
		want   string
	}{
		"the remote gh marked as the base": {
			config: remoteConfig(
				"remote.origin.url", "git@github.com:contributor/repo.git",
				"remote.parent.url", "git@github.com:owner/repo.git",
				"remote.parent.gh-resolved", "base",
			),
			want: "parent",
		},
		"a remote named upstream": {
			config: remoteConfig(
				"remote.origin.url", "git@github.com:contributor/repo.git",
				"remote.upstream.url", "git@github.com:owner/repo.git",
			),
			want: upstreamName,
		},
		"gh's mark outranks upstream": {
			config: remoteConfig(
				"remote.upstream.url", "git@github.com:owner/repo.git",
				"remote.parent.url", "git@github.com:owner/repo.git",
				"remote.parent.gh-resolved", "base",
			),
			want: "parent",
		},
		// gh writes an OWNER/REPO resolution for a remote it did not choose as
		// the base.
		"gh's resolution of another repository": {
			config: remoteConfig(
				"remote.origin.url", "git@github.com:contributor/repo.git",
				"remote.origin.gh-resolved", "owner/repo",
			),
			want: gitrepo.DefaultRemote,
		},
		"a remote whose name holds a dot": {
			config: remoteConfig(
				"remote.my.upstream.url", "git@github.com:owner/repo.git",
				"remote.my.upstream.gh-resolved", "base",
			),
			want: "my.upstream",
		},
		"only origin": {
			config: remoteConfig("remote.origin.url", "git@github.com:owner/repo.git"),
			want:   gitrepo.DefaultRemote,
		},
		"no remote at all":         {config: reply{err: errNoRef}, want: gitrepo.DefaultRemote},
		"a config git cannot read": {config: reply{err: errNotARepository}, want: gitrepo.DefaultRemote},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := gitrepo.At(fakeRunner(t, map[string]reply{readRemotes: tt.config}), workDir)

			// Act
			got := repo.BaseRemote(t.Context())

			// Assert
			if got != tt.want {
				t.Errorf("BaseRemote = %q, want %q", got, tt.want)
			}
		})
	}
}

// forkClone is the remote configuration of a fork's clone: origin is the
// fork, upstream the repository it was forked from.
func forkClone() reply {
	return remoteConfig(
		"remote.origin.url", "git@github.com:contributor/repo.git",
		"remote.upstream.url", "git@github.com:owner/repo.git",
	)
}

func TestReadBranchReadsTheBaseOnTheBaseRemote(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := with(featureBranch(), map[string]reply{
		readRemotes: forkClone(),
		"git -C /work symbolic-ref --quiet --short refs/remotes/upstream/HEAD": {out: []byte("upstream/main\n")},
		logCommits + "upstream/main..HEAD":                                     {out: []byte(firstHash + "\x00fix: one\x00")},
		baseAge + "upstream/main":                                              {out: []byte(baseUpdatedISO + "\n")},
	})

	// Act
	branch, err := gitrepo.At(fakeRunner(t, replies), workDir).ReadBranch(t.Context())

	// Assert
	if err != nil || branch.Base != "upstream/main" || branch.BaseRemote != upstreamName || len(branch.Commits) != 1 {
		t.Errorf("ReadBranch = %+v, %v; want upstream/main read on upstream", branch, err)
	}
}

func TestChangedPathsReadsTheBaseRemotesCopy(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		readRemotes: forkClone(),
		"git -C /work rev-parse --verify --quiet refs/remotes/upstream/main": {out: []byte("0123abcd\n")},
		"git -C /work diff --name-only -z --no-renames upstream/main...HEAD": {out: []byte(goFile + "\x00")},
	}), workDir)

	// Act
	paths, err := repo.ChangedPaths(t.Context(), "main")

	// Assert
	if err != nil || !slices.Equal(paths, []string{goFile}) {
		t.Errorf("ChangedPaths = %q, %v; want %s against upstream/main", paths, err, goFile)
	}
}

func TestCodeOwnersAtReadsTheDialectsFileAtTheBaseRemote(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := gitrepo.At(fakeRunner(t, map[string]reply{
		readRemotes: forkClone(),
		"git -C /work rev-parse --verify --quiet refs/remotes/upstream/main": {out: []byte("0123abcd\n")},
		"git -C /work rev-parse --verify --quiet upstream/main^{commit}":     {out: []byte("0123abcd\n")},
		"git -C /work show upstream/main:.github/CODEOWNERS":                 {out: []byte("*.go @owner/go\n")},
	}), workDir)

	// Act
	file, found, err := repo.CodeOwnersAt(t.Context(), "main", codeowners.GitHub)

	// Assert
	owners := file.OwnersOf([]string{goFile})
	if err != nil || !found || !slices.Equal(owners.Teams, []string{"owner/go"}) {
		t.Errorf("CodeOwnersAt = %+v, %v, %v; want upstream/main's rules", owners, found, err)
	}
}
