// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks_test

import (
	"slices"
	"testing"
	"testing/fstest"

	"github.com/jacob-delgado/workflow/internal/hooks"
)

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

func TestExistingHooksTellLefthooksShimFromAHookThatMentionsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// lefthook's shim defines and calls call_lefthook; a hand-written hook may
	// mention lefthook in a comment or run it as one step among others.
	mentions := "#!/bin/sh\nset -e\n# moved here from lefthook\nmake lint\n"
	dir := fstest.MapFS{
		preCommit: executable(mentions),
		"post-checkout": executable("#!/bin/sh\n\nif [ \"$LEFTHOOK\" = \"0\" ]; then\n  exit 0\nfi\n\n" +
			"call_lefthook()\n{\n  lefthook \"$@\"\n}\n\ncall_lefthook run \"post-checkout\" \"$@\"\n"),
		"post-merge": executable("#!/bin/sh\n# lefthook_version: 2\nlefthook run post-merge\n"),
	}

	// Act
	found := hooks.ExistingHooks(dir, "linux")

	// Assert
	if len(found) != 1 || found[0].Name != preCommit || found[0].Script != mentions {
		t.Errorf("ExistingHooks = %+v, want only the hook that mentions lefthook", found)
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
