// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import (
	"context"
	"fmt"
	"strings"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// Status letters git uses beyond the ordinary ones.
const (
	unchanged = ' '
	untracked = '?'
	ignored   = '!'
	unmerged  = 'U'
	added     = 'A'
	deleted   = 'D'
	renamed   = 'R'
	copied    = 'C'
)

// minimumEntry is the shortest status entry: two letters, a space and a path of
// at least one character.
const minimumEntry = 4

// Change is one changed file, as git status reports it.
type Change struct {
	// Path is where the file is now, exactly as git wrote it. It is the path to
	// hand back to git, so it is never altered; anything showing it on screen
	// must neutralize it first, because a file name may hold any byte but / and
	// NUL — escape sequences included.
	Path string
	// OriginalPath is where a renamed or copied file came from.
	OriginalPath string
	// Staged is what the index holds against HEAD, and Unstaged what the work
	// tree holds against the index: git's two status letters, a space meaning
	// no change.
	Staged   byte
	Unstaged byte
}

// Kind names the kind of change in a word, for showing rather than git's letter.
func (c Change) Kind() string {
	if c.Conflicted() {
		return "conflicted"
	}

	if c.Staged == untracked || c.Unstaged == untracked {
		return "untracked"
	}

	words := map[byte]string{added: "new", deleted: "deleted", renamed: "renamed", copied: "copied"}

	letter := c.Staged
	if letter == unchanged {
		letter = c.Unstaged
	}

	if word, named := words[letter]; named {
		return word
	}

	return "modified"
}

// IsStaged reports changes in the index, waiting to be committed.
func (c Change) IsStaged() bool {
	return !c.Conflicted() && c.Staged != unchanged && c.Staged != untracked && c.Staged != ignored
}

// HasUnstaged reports changes in the work tree not yet in the index, including
// a file git does not track at all.
func (c Change) HasUnstaged() bool {
	return !c.Conflicted() && c.Unstaged != unchanged
}

// Conflicted reports a file a merge left unresolved: either side unmerged, or
// both sides adding or deleting it.
func (c Change) Conflicted() bool {
	return c.Staged == unmerged || c.Unstaged == unmerged ||
		(c.Staged == c.Unstaged && (c.Staged == added || c.Staged == deleted))
}

// Untracked reports a file git does not track at all, whose whole content is
// new — so it has no diff against HEAD until it is staged.
func (c Change) Untracked() bool {
	return c.Staged == untracked || c.Unstaged == untracked
}

// paths is every path the change touches: both, for a rename.
func (c Change) paths() []string {
	if c.OriginalPath == "" {
		return []string{c.Path}
	}

	return []string{c.Path, c.OriginalPath}
}

// untrackedNote stands in for the diff of an untracked file, which git cannot
// show against HEAD until the file is staged.
const untrackedNote = "new file, not yet staged — stage it to see its diff"

// Diff reads a change's diff against HEAD, line by line and sanitized, so it can
// be read before staging. An untracked file has no such diff — it is whole and
// new — so a note stands in for it.
func (r Repository) Diff(ctx context.Context, change Change) ([]string, error) {
	if change.Untracked() {
		return []string{untrackedNote}, nil
	}

	args := append([]string{"-C", r.dir, literalPathspecs, "diff", "HEAD", "--"}, change.paths()...)

	out, err := r.run(ctx, gitProgram, args...)
	if err != nil {
		return nil, readFailure(ctx, r.run, r.dir, "reading the diff of "+change.Path, err)
	}

	body := text(out)
	if body == "" {
		return nil, nil
	}

	var lines []string

	//nolint:modernize // SplitSeq returns a range-over-func iterator, which crashes gobco.
	for _, line := range strings.Split(body, "\n") {
		lines = append(lines, sanitize.Text(line))
	}

	return lines, nil
}

// Status lists every changed file in the work tree, untracked files included.
func (r Repository) Status(ctx context.Context) ([]Change, error) {
	out, err := r.run(ctx, "git", "-C", r.dir, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, readFailure(ctx, r.run, r.dir, "reading the status of "+r.dir, err)
	}

	return parseStatus(string(out)), nil
}

// parseStatus reads porcelain v1 with -z: NUL-separated "XY path" entries, where
// a rename or copy is followed by one more entry holding the original path. -z
// is what keeps a path with a space, a quote or a newline in it intact.
func parseStatus(out string) []Change {
	entries := strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
	changes := make([]Change, 0, len(entries))

	for index := 0; index < len(entries); index++ {
		entry := entries[index]
		if len(entry) < minimumEntry {
			continue
		}

		change := Change{Path: entry[3:], OriginalPath: "", Staged: entry[0], Unstaged: entry[1]}

		if moved(change) && index+1 < len(entries) {
			index++
			change.OriginalPath = entries[index]
		}

		changes = append(changes, change)
	}

	return changes
}

// moved reports a rename or copy, whose entry carries a second path.
func moved(change Change) bool {
	return change.Staged == renamed || change.Staged == copied ||
		change.Unstaged == renamed || change.Unstaged == copied
}

// literalPathspecs turns off git's globbing and pathspec magic, so a name is
// taken as itself, not as a pattern.
const literalPathspecs = "--literal-pathspecs"

// Stage puts a file's changes in the index — additions, edits and deletions
// alike, which is what --all is for. --literal-pathspecs turns off globbing and
// pathspec magic, so a name like `[id].tsx` stages only itself; `--` alone ends
// options but does not stop git reading the path as a pattern.
func (r Repository) Stage(ctx context.Context, change Change) error {
	args := append([]string{"-C", r.dir, literalPathspecs, "add", "--all", "--"}, change.paths()...)

	_, err := r.run(ctx, "git", args...)
	if err != nil {
		return fmt.Errorf("staging %s: %w", sanitize.Line(change.Path), err)
	}

	return nil
}

// Unstage takes a file's changes out of the index and leaves the work tree
// alone. Before the first commit there is no HEAD to restore from, so the file
// is removed from the index instead.
func (r Repository) Unstage(ctx context.Context, change Change) error {
	args := append([]string{"-C", r.dir, literalPathspecs, "restore", "--staged", "--"}, change.paths()...)

	_, err := r.run(ctx, "git", "-C", r.dir, "rev-parse", "--verify", "--quiet", "HEAD")
	if err != nil {
		args = append([]string{"-C", r.dir, literalPathspecs, "rm", "--cached", "--quiet", "--"}, change.paths()...)
	}

	_, err = r.run(ctx, "git", args...)
	if err != nil {
		return fmt.Errorf("unstaging %s: %w", sanitize.Line(change.Path), err)
	}

	return nil
}
