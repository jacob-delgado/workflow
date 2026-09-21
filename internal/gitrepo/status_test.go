// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// errIndexLocked stands in for git refusing to touch a locked index.
var errIndexLocked = errors.New("fatal: Unable to create '.git/index.lock': File exists")

// renamedFrom and renamedTo are where the renamed file in these fixtures was,
// and is.
const (
	renamedFrom = "a.txt"
	renamedTo   = "b.txt"
)

// statusCommand is the exact status invocation Status runs.
const statusCommand = "git -C /work status --porcelain=v1 -z --untracked-files=all"

func TestStatusReadsTheWorkTree(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		answer  reply
		want    []gitrepo.Change
		wantErr error
	}{
		// Real porcelain output: a staged rename that was then edited, a
		// deletion not yet staged, and an untracked file with a space in its name.
		"every kind of change": {
			answer: reply{out: []byte("RM b.txt\x00a.txt\x00 D gone.txt\x00?? new file.txt\x00")},
			want: []gitrepo.Change{
				{Path: renamedTo, OriginalPath: renamedFrom, Staged: 'R', Unstaged: 'M'},
				{Path: "gone.txt", OriginalPath: "", Staged: ' ', Unstaged: 'D'},
				{Path: "new file.txt", OriginalPath: "", Staged: '?', Unstaged: '?'},
			},
		},
		"a clean tree":  {answer: reply{}, want: nil},
		"git's failure": {answer: reply{err: errIndexLocked}, want: nil, wantErr: errIndexLocked},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run := fakeRunner(t, map[string]reply{
				statusCommand: tt.answer,
				showToplevel:  {out: []byte("/work\n")},
			})

			// Act
			changes, err := gitrepo.At(run, workDir).Status(t.Context())

			// Assert
			if !errors.Is(err, tt.wantErr) || !slices.Equal(changes, tt.want) {
				t.Errorf("Status = %+v, %v; want %+v, %v", changes, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestAChangeSaysWhereItStands(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change                       gitrepo.Change
		staged, unstaged, conflicted bool
	}{
		"staged only":   {change: gitrepo.Change{Staged: 'M', Unstaged: ' '}, staged: true},
		"unstaged only": {change: gitrepo.Change{Staged: ' ', Unstaged: 'M'}, unstaged: true},
		"both":          {change: gitrepo.Change{Staged: 'A', Unstaged: 'M'}, staged: true, unstaged: true},
		"untracked":     {change: gitrepo.Change{Staged: '?', Unstaged: '?'}, unstaged: true},
		"both modified": {change: gitrepo.Change{Staged: 'U', Unstaged: 'U'}, conflicted: true},
		"both added":    {change: gitrepo.Change{Staged: 'A', Unstaged: 'A'}, conflicted: true},
		"both deleted":  {change: gitrepo.Change{Staged: 'D', Unstaged: 'D'}, conflicted: true},
		"deleted by us": {change: gitrepo.Change{Staged: 'D', Unstaged: 'U'}, conflicted: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := [3]bool{tt.change.IsStaged(), tt.change.HasUnstaged(), tt.change.Conflicted()}

			// Assert
			if want := [3]bool{tt.staged, tt.unstaged, tt.conflicted}; got != want {
				t.Errorf("staged, unstaged, conflicted = %v, want %v", got, want)
			}
		})
	}
}

func TestStageAddsEveryPathTheChangeTouches(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change gitrepo.Change
		want   string
	}{
		// A rename is two paths: staging only the new one would leave the old
		// one's deletion behind.
		"a rename": {
			change: gitrepo.Change{Path: renamedTo, OriginalPath: renamedFrom},
			want:   "git -C /work --literal-pathspecs add --all -- b.txt a.txt",
		},
		"a deletion": {
			change: gitrepo.Change{Path: "gone.txt", Unstaged: 'D'},
			want:   "git -C /work --literal-pathspecs add --all -- gone.txt",
		},
		// A bracketed name is a glob to git unless pathspecs are literal, so it
		// would stage i.tsx and d.tsx beside it.
		"a bracketed name": {
			change: gitrepo.Change{Path: "[id].tsx", Unstaged: '?'},
			want:   "git -C /work --literal-pathspecs add --all -- [id].tsx",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run, ran := recordingRunner(t, map[string]reply{tt.want: {}})

			// Act
			err := gitrepo.At(run, workDir).Stage(t.Context(), tt.change)

			// Assert
			if err != nil || !slices.Equal(*ran, []string{tt.want}) {
				t.Errorf("Stage ran %q and returned %v, want %q", *ran, err, tt.want)
			}
		})
	}
}

func TestUnstageTakesTheChangeOutOfTheIndexWithoutTouchingTheWorkTree(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		head   reply
		change gitrepo.Change
		want   string
	}{
		"after a commit, the index is restored from HEAD": {
			head:   reply{out: []byte("abc123\n")},
			change: gitrepo.Change{Path: renamedTo, OriginalPath: renamedFrom},
			want:   "git -C /work --literal-pathspecs restore --staged -- b.txt a.txt",
		},
		// With no commit there is nothing to restore from: restore --staged fails.
		"before the first commit, the file leaves the index": {
			head:   reply{err: errDetachedRead},
			change: gitrepo.Change{Path: "new.go"},
			want:   "git -C /work --literal-pathspecs rm --cached --quiet -- new.go",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run, ran := recordingRunner(t, map[string]reply{verifyHead: tt.head, tt.want: {}})

			// Act
			err := gitrepo.At(run, workDir).Unstage(t.Context(), tt.change)

			// Assert
			if err != nil || !slices.Equal(*ran, []string{verifyHead, tt.want}) {
				t.Errorf("Unstage ran %q and returned %v, want %q after checking HEAD", *ran, err, tt.want)
			}
		})
	}
}

func TestStageReportsGitsFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	run := fakeRunner(t, map[string]reply{"git -C /work --literal-pathspecs add --all -- x.go": {err: errIndexLocked}})

	// Act
	err := gitrepo.At(run, workDir).Stage(t.Context(), gitrepo.Change{Path: "x.go"})

	// Assert
	if !errors.Is(err, errIndexLocked) {
		t.Errorf("Stage returned %v, want git's error", err)
	}
}

func TestUnstageReportsGitsFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	run := fakeRunner(t, map[string]reply{
		verifyHead: {out: []byte("abc\n")},
		"git -C /work --literal-pathspecs restore --staged -- x.go": {err: errIndexLocked},
	})

	// Act
	err := gitrepo.At(run, workDir).Unstage(t.Context(), gitrepo.Change{Path: "x.go"})

	// Assert
	if !errors.Is(err, errIndexLocked) {
		t.Errorf("Unstage returned %v, want git's error", err)
	}
}

func TestAStagingFailureNamesTheFileInTextThatIsSafeToShow(t *testing.T) {
	t.Parallel()

	// A file name may hold any byte but / and NUL. git is handed the name as it
	// is; an error is read by a person, and is handed the name made safe.
	const awkward = "a\x1b]0;owned\x07\nb.go"

	cases := map[string]func(run gitrepo.Runner) error{
		"staging": func(run gitrepo.Runner) error {
			return gitrepo.At(run, workDir).Stage(t.Context(), gitrepo.Change{Path: awkward})
		},
		"unstaging": func(run gitrepo.Runner) error {
			return gitrepo.At(run, workDir).Unstage(t.Context(), gitrepo.Change{Path: awkward})
		},
	}

	for name, act := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run := fakeRunner(t, map[string]reply{
				verifyHead: {out: []byte("abc\n")},
				"git -C /work --literal-pathspecs add --all -- " + awkward:        {err: errIndexLocked},
				"git -C /work --literal-pathspecs restore --staged -- " + awkward: {err: errIndexLocked},
			})

			// Act
			err := act(run)

			// Assert
			if !errors.Is(err, errIndexLocked) || strings.ContainsAny(err.Error(), "\x1b\n") ||
				!strings.Contains(err.Error(), "b.go") {
				t.Errorf("the error reads %q, want git's error naming the file on one safe line", err)
			}
		})
	}
}

func TestAChangeNamesItsKindInAWord(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change gitrepo.Change
		want   string
	}{
		"a staged edit":     {change: gitrepo.Change{Staged: 'M', Unstaged: ' '}, want: "modified"},
		"a work-tree edit":  {change: gitrepo.Change{Staged: ' ', Unstaged: 'M'}, want: "modified"},
		"a new file":        {change: gitrepo.Change{Staged: 'A', Unstaged: ' '}, want: "new"},
		"a deleted file":    {change: gitrepo.Change{Staged: 'D', Unstaged: ' '}, want: "deleted"},
		"a renamed file":    {change: gitrepo.Change{Staged: 'R', Unstaged: ' '}, want: "renamed"},
		"a copied file":     {change: gitrepo.Change{Staged: 'C', Unstaged: ' '}, want: "copied"},
		"an untracked file": {change: gitrepo.Change{Staged: '?', Unstaged: '?'}, want: "untracked"},
		"a conflicted file": {change: gitrepo.Change{Staged: 'U', Unstaged: 'U'}, want: "conflicted"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := tt.change.Kind(); got != tt.want {
				t.Errorf("Kind() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusReportsADirectoryOutsideARepository(t *testing.T) {
	t.Parallel()

	// Arrange
	run := fakeRunner(t, map[string]reply{
		"git -C /work status --porcelain=v1 -z --untracked-files=all": {err: errIndexLocked},
		showToplevel: {err: errIndexLocked},
	})

	// Act
	_, err := gitrepo.At(run, workDir).Status(t.Context())

	// Assert
	if !errors.Is(err, gitrepo.ErrNotARepository) {
		t.Errorf("Status returned %v, want ErrNotARepository", err)
	}
}

func TestDiffReadsAFilesChangeAgainstHead(t *testing.T) {
	t.Parallel()

	// Arrange
	body := "diff --git a/internal/config/redact.go b/internal/config/redact.go\n" +
		"@@ -1,3 +1,4 @@\n context\n-old line\n+new line\n"
	replies := map[string]reply{
		"git -C /work --literal-pathspecs diff HEAD -- internal/config/redact.go": {out: []byte(body)},
	}
	change := gitrepo.Change{Path: "internal/config/redact.go", Staged: 'M', Unstaged: ' '}

	// Act
	lines, err := gitrepo.At(fakeRunner(t, replies), workDir).Diff(t.Context(), change)

	// Assert
	want := []string{
		"diff --git a/internal/config/redact.go b/internal/config/redact.go",
		"@@ -1,3 +1,4 @@", " context", "-old line", "+new line",
	}
	if err != nil || !slices.Equal(lines, want) {
		t.Errorf("Diff = %q, %v, want %q", lines, err, want)
	}
}

func TestDiffOfARenamePassesBothPaths(t *testing.T) {
	t.Parallel()

	// Arrange
	body := "diff --git a/former.go b/moved.go\nrename from former.go\nrename to moved.go\n"
	replies := map[string]reply{
		"git -C /work --literal-pathspecs diff HEAD -- moved.go former.go": {out: []byte(body)},
	}
	change := gitrepo.Change{Path: "moved.go", OriginalPath: "former.go", Staged: 'R', Unstaged: ' '}

	// Act
	lines, err := gitrepo.At(fakeRunner(t, replies), workDir).Diff(t.Context(), change)

	// Assert
	if err != nil || len(lines) != 3 || lines[0] != "diff --git a/former.go b/moved.go" {
		t.Errorf("Diff = %q, %v, want the rename's diff over both paths", lines, err)
	}
}

func TestDiffOfANoTextualChangeIsEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	// A mode-only change prints only the header lines, which git trims to nothing.
	replies := map[string]reply{"git -C /work --literal-pathspecs diff HEAD -- mode.go": {out: []byte("")}}
	change := gitrepo.Change{Path: "mode.go", Staged: 'M', Unstaged: ' '}

	// Act
	lines, err := gitrepo.At(fakeRunner(t, replies), workDir).Diff(t.Context(), change)

	// Assert
	if err != nil || lines != nil {
		t.Errorf("Diff = %q, %v, want no lines", lines, err)
	}
}

func TestDiffOfAnUntrackedFileIsANote(t *testing.T) {
	t.Parallel()

	// Arrange
	// An untracked file is whole and new, so no git command runs.
	change := gitrepo.Change{Path: "fresh.go", Staged: '?', Unstaged: '?'}

	// Act
	lines, err := gitrepo.At(fakeRunner(t, map[string]reply{}), workDir).Diff(t.Context(), change)

	// Assert
	if err != nil || len(lines) != 1 || !strings.Contains(lines[0], "new file") {
		t.Errorf("Diff = %q, %v, want a new-file note", lines, err)
	}
}

func TestDiffReportsGitsFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	change := gitrepo.Change{Path: "gone.go", Staged: 'M', Unstaged: ' '}
	replies := map[string]reply{
		"git -C /work --literal-pathspecs diff HEAD -- gone.go": {err: errIndexLocked},
		showToplevel: {err: errIndexLocked},
	}

	// Act
	_, err := gitrepo.At(fakeRunner(t, replies), workDir).Diff(t.Context(), change)

	// Assert
	if !errors.Is(err, gitrepo.ErrNotARepository) {
		t.Errorf("Diff returned %v, want ErrNotARepository", err)
	}
}
