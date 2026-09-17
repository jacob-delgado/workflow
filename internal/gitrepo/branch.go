// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// commitLimit caps how many of a branch's commits are read. A branch under
// review has a handful; one with hundreds is a merge nobody reads in a pane.
const commitLimit = 200

// logSeparator is what git writes after a commit's hash and, under -z, after its
// subject. A subject may hold any byte but this one: git refuses a commit
// message with a NUL in it, which is what makes it safe to split on.
const logSeparator = "\x00"

// logFormat asks for a commit's abbreviated hash and its subject.
const logFormat = "--format=%h%x00%s"

// Commit is one commit on a branch.
type Commit struct {
	Hash    string
	Subject string
}

// Branch is where the checked-out branch stands.
type Branch struct {
	Name     string
	Detached bool
	// Head is the commit checked out, or empty before the first commit.
	Head string
	// Upstream is the remote branch it pushes to, or empty if never pushed.
	Upstream string
	// Ahead and Behind count commits against the upstream.
	Ahead, Behind int
	// Base is the branch this one would merge into — origin's default branch
	// wherever that can be found.
	Base string
	// Commits are those on this branch and not on Base, oldest first.
	Commits []Commit
}

// Pushed reports whether origin holds this branch, under its own name, with
// every commit. The name matters: a branch created from origin/main without
// --no-track tracks origin/main, is ahead of it by nothing, and is not on the
// remote at all.
func (b Branch) Pushed() bool {
	return b.Name != "" && b.Upstream == "origin/"+b.Name && b.Ahead == 0
}

// ReadBranch reads where the checked-out branch stands. Only failing to read the
// branch at all is an error: no commits, no upstream and no base are ordinary
// states for a repository to be in, and each is left empty.
func ReadBranch(ctx context.Context, run Runner, dir string) (Branch, error) {
	name, err := run(ctx, "git", "-C", dir, "branch", "--show-current")
	if err != nil {
		return Branch{}, fmt.Errorf("reading the current branch of %s: %w", dir, err)
	}

	git := func(args ...string) string {
		return optional(ctx, run, append([]string{"-C", dir}, args...)...)
	}

	branch := Branch{
		Name:     text(name),
		Detached: text(name) == "",
		Head:     git("rev-parse", "HEAD"),
		Upstream: git("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"),
		Ahead:    0,
		Behind:   0,
		Base:     base(git),
		Commits:  nil,
	}

	if branch.Upstream != "" {
		branch.Behind, branch.Ahead = counts(git("rev-list", "--left-right", "--count", "@{upstream}...HEAD"))
	}

	if branch.Base != "" && branch.Head != "" {
		branch.Commits = parseLog(git("log", "-z", "--reverse", "--max-count="+strconv.Itoa(commitLimit),
			logFormat, branch.Base+"..HEAD"))
	}

	return branch, nil
}

// optional runs git and returns its trimmed output, or "" if it failed: for the
// questions whose failure is an answer, such as a branch with no upstream.
func optional(ctx context.Context, run Runner, args ...string) string {
	out, err := run(ctx, "git", args...)
	if err != nil {
		return ""
	}

	return text(out)
}

// base finds the branch work merges into: what origin says its default is, or
// failing that the conventional names, remote first.
func base(git func(...string) string) string {
	if origin := git("symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); origin != "" {
		return origin
	}

	candidates := []struct{ ref, name string }{
		{ref: "refs/remotes/origin/main", name: "origin/main"},
		{ref: "refs/remotes/origin/master", name: "origin/master"},
		{ref: "refs/heads/main", name: "main"},
		{ref: "refs/heads/master", name: "master"},
	}

	for _, candidate := range candidates {
		if git("rev-parse", "--verify", "--quiet", candidate.ref) != "" {
			return candidate.name
		}
	}

	return ""
}

// counts reads rev-list --left-right --count: the left side's count, a tab, the
// right side's. Anything else reads as zero and zero.
func counts(out string) (int, int) {
	left, right, found := strings.Cut(out, "\t")
	if !found {
		return 0, 0
	}

	leftCount, leftErr := strconv.Atoi(left)
	rightCount, rightErr := strconv.Atoi(right)

	if leftErr != nil || rightErr != nil {
		return 0, 0
	}

	return leftCount, rightCount
}

// parseLog reads commits written as logFormat under -z: a hash, a subject, a
// hash, a subject. A subject is neutralized here, where it enters the program:
// anyone able to put a commit on the base branch wrote it. A hash is only ever
// hex, so a record that opens with anything else is not one git wrote, and is
// left out.
func parseLog(out string) []Commit {
	var commits []Commit

	fields := strings.Split(out, logSeparator)

	for index := 0; index+1 < len(fields); index += 2 {
		hash, subject := fields[index], fields[index+1]
		if !isHex(hash) {
			continue
		}

		commits = append(commits, Commit{Hash: hash, Subject: sanitize.Text(subject)})
	}

	return commits
}

// isHex reports text made of hexadecimal digits and nothing else.
func isHex(text string) bool {
	if text == "" {
		return false
	}

	for _, digit := range text {
		if !strings.ContainsRune("0123456789abcdefABCDEF", digit) {
			return false
		}
	}

	return true
}

// CreateBranch creates a branch at start and switches to it, or at HEAD when
// start is empty.
//
// --no-track matters when start is a remote branch such as origin/main: without
// it git makes origin/main the new branch's upstream, so the branch reads as
// ahead of main rather than as never pushed.
func CreateBranch(ctx context.Context, run Runner, dir, name, start string) error {
	args := []string{"-C", dir, "switch", "--create", name}
	if start != "" {
		args = append(args, "--no-track", start)
	}

	_, err := run(ctx, "git", args...)
	if err != nil {
		return fmt.Errorf("creating branch %s: %w", name, err)
	}

	return nil
}

// PushCommand pushes a branch to origin and makes it the upstream.
//
// Prompts are off: nobody can answer a credential prompt from inside the
// interface, so git must fail with a reason rather than wait forever for input
// that is never coming.
func PushCommand(dir, branch string) proc.Command {
	return proc.Command{
		Dir:  dir,
		Name: "git",
		Args: []string{"push", "--set-upstream", "origin", branch},
		Env:  []string{"GIT_TERMINAL_PROMPT=0"},
	}
}

// CommitCommand commits the index with the message in a file. It is a plain
// git commit, so the repository's hooks run exactly as they do in a terminal.
func CommitCommand(dir, messageFile string) proc.Command {
	return proc.Command{Dir: dir, Name: "git", Args: []string{"commit", "--file", messageFile}, Env: nil}
}

// HooksDir is where git looks for this repository's hooks, which core.hooksPath
// can move anywhere.
func HooksDir(ctx context.Context, run Runner, dir string) (string, error) {
	out, err := run(ctx, "git", "-C", dir, "rev-parse", "--git-path", "hooks")
	if err != nil {
		return "", fmt.Errorf("finding the hooks directory of %s: %w", dir, err)
	}

	path := text(out)
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}

	return path, nil
}
