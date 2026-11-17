// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks_test

import (
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/jacob-delgado/workflow/internal/hooks"
)

// offeredFrom are the hooks Survey offers from hooksDir in a repository that
// does not configure lefthook.
func offeredFrom(hooksDir fs.FS, goos string) []hooks.GitHook {
	return hooks.Survey(hooksDir, fstest.MapFS{}, goos).Hooks()
}

func TestTheHooksOfferedAreTheOnesGitWouldRun(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := fstest.MapFS{
		preCommit:                executable(simplePreCommit),
		commitMsg:                executable(complexCommitMsg),
		"pre-push.sample":        executable("#!/bin/sh\n"),
		prePush:                  {Data: []byte("#!/bin/sh\nexit 1\n"), Mode: 0o644},
		postCheckout:             executable("#!/bin/sh\n# lefthook_version: 2\ncall_lefthook run post-checkout\n"),
		"prepare-commit-msg.old": executable("#!/bin/sh\n"),
		"notes.txt":              executable("not a hook"),
	}

	// Act
	found := offeredFrom(dir, "linux")

	// Assert
	// Git's own order, not the directory's: commit-msg after pre-commit. Not
	// a sample, not a file without the executable bit (git skips those), not
	// a hook lefthook already installed, not a leftover .old.
	names := make([]string, 0, len(found))
	for _, each := range found {
		names = append(names, each.Name)
	}

	if want := []string{preCommit, commitMsg}; !slices.Equal(names, want) {
		t.Fatalf("offered %q, want %q", names, want)
	}

	if found[0].Script != simplePreCommit {
		t.Errorf("the script was not read whole: %q", found[0].Script)
	}
}

func TestTheHooksOfferedTellLefthooksShimFromAHookThatMentionsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// lefthook's shim defines and calls call_lefthook; a hand-written hook may
	// mention lefthook in a comment or run it as one step among others.
	mentions := "#!/bin/sh\nset -e\n# moved here from lefthook\nmake lint\n"
	dir := fstest.MapFS{
		preCommit: executable(mentions),
		postCheckout: executable("#!/bin/sh\n\nif [ \"$LEFTHOOK\" = \"0\" ]; then\n  exit 0\nfi\n\n" +
			"call_lefthook()\n{\n  lefthook \"$@\"\n}\n\ncall_lefthook run \"post-checkout\" \"$@\"\n"),
		"post-merge": executable("#!/bin/sh\n# lefthook_version: 2\nlefthook run post-merge\n"),
	}

	// Act
	found := offeredFrom(dir, "linux")

	// Assert
	if len(found) != 1 || found[0].Name != preCommit || found[0].Script != mentions {
		t.Errorf("offered %+v, want only the hook that mentions lefthook", found)
	}
}

func TestTheHooksOfferedNeedNoExecutableBitOnWindows(t *testing.T) {
	t.Parallel()

	// Arrange
	// Go reports no executable bit for any file on Windows, so requiring one
	// would find no hooks there; a plain, non-executable file must count.
	dir := fstest.MapFS{
		preCommit: {Data: []byte(simplePreCommit), Mode: 0o644},
	}

	// Act
	found := offeredFrom(dir, "windows")

	// Assert
	if len(found) != 1 || found[0].Name != preCommit {
		t.Errorf("offered %+v on Windows, want the non-executable pre-commit found", found)
	}
}

// lefthookYML is the configuration lefthook reads first.
const lefthookYML = "lefthook.yml"

func TestSurveyOffersTheHooksOfARepositoryLefthookDoesNotConfigure(t *testing.T) {
	t.Parallel()

	withHooks := fstest.MapFS{preCommit: executable(simplePreCommit)}
	configured := fstest.MapFS{lefthookYML: {Data: []byte(preCommit + ":\n"), Mode: 0o644}}

	cases := map[string]struct {
		hooksDir, repo fstest.MapFS
		offering       hooks.Offering
		offered        []hooks.GitHook
	}{
		"hooks and no configuration": {
			hooksDir: withHooks, repo: fstest.MapFS{}, offering: hooks.OfferingHooks,
			offered: []hooks.GitHook{{Name: preCommit, Script: simplePreCommit}},
		},
		"hooks in a repository lefthook configures": {
			hooksDir: withHooks, repo: configured, offering: hooks.OfferingConfigured,
		},
		"no hooks": {hooksDir: fstest.MapFS{}, repo: fstest.MapFS{}, offering: hooks.OfferingNothing},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			setup := hooks.Survey(tt.hooksDir, tt.repo, "linux")

			// Assert
			if setup.Offering() != tt.offering || !slices.Equal(setup.Hooks(), tt.offered) {
				t.Errorf("Survey = %v offering %+v, want %v offering %+v",
					setup.Offering(), setup.Hooks(), tt.offering, tt.offered)
			}
		})
	}
}

// A Setup made of nothing, as a seam answers beside its failure, offers
// nothing.
func TestTheZeroSetupOffersNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	var setup hooks.Setup

	// Act & Assert
	if setup.Offering() != hooks.OfferingNothing || len(setup.Hooks()) != 0 {
		t.Errorf("the zero Setup offers %v: %+v; want nothing", setup.Offering(), setup.Hooks())
	}
}
