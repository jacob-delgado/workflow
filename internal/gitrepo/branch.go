// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// commitLimit caps how many of a branch's commits are read. A branch under
// review has a handful; one with hundreds is a merge nobody reads in a pane.
const commitLimit = 200

// gitProgram is the program every command here runs.
const gitProgram = "git"

// noTerminalPrompt turns off git's credential prompts: nobody can answer one
// from inside the interface, so a command that needs a credential must fail
// rather than wait for input that is never coming.
const noTerminalPrompt = "GIT_TERMINAL_PROMPT=0"

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
	// BaseUpdated is when the base last moved, so the interface can say how
	// stale the branch it started from is. Zero when it cannot be read.
	BaseUpdated time.Time
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
		return Branch{}, readFailure(ctx, run, dir, "reading the current branch of "+dir, err)
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

	err = shownAsTheyAre(branch.Name, branch.Upstream, branch.Base)
	if err != nil {
		return Branch{}, err
	}

	if branch.Upstream != "" {
		branch.Behind, branch.Ahead = counts(git("rev-list", "--left-right", "--count", "@{upstream}...HEAD"))
	}

	if branch.Base != "" && branch.Head != "" {
		// git applies --max-count before --reverse, so capping in the command
		// would keep the newest commits and lose the branch's first — the one
		// the pull request titles itself with. Reverse the whole range, then cap
		// the oldest-first result here.
		commits := parseLog(git("log", "-z", "--reverse", logFormat, branch.Base+"..HEAD"))
		if len(commits) > commitLimit {
			commits = commits[:commitLimit]
		}

		branch.Commits = commits
		branch.BaseUpdated = parseTime(git("log", "-1", "--format=%cI", branch.Base))
	}

	return branch, nil
}

// parseTime reads an RFC 3339 timestamp, returning the zero time for anything
// git did not answer with — the base's age is a nicety, never a failure.
func parseTime(text string) time.Time {
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return time.Time{}
	}

	return parsed
}

// ErrUnshowableName reports a branch whose name cannot be shown as it is.
var ErrUnshowableName = errors.New("a branch name holds characters that cannot be shown as they are")

// shownAsTheyAre refuses a ref whose name would not be drawn as it is: git
// allows a direction override or a character with no width in one. A name is
// handed to git and to the forge as well as drawn, so it cannot be cleaned for
// one and kept for the others; a branch like that is not worked on.
func shownAsTheyAre(refs ...string) error {
	for _, ref := range refs {
		if shown := sanitize.Line(ref); shown != ref {
			return fmt.Errorf("%w: %s", ErrUnshowableName, shown)
		}
	}

	return nil
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

// LocalBranches lists the repository's local branches, most recently committed
// to first, so a switcher offers the ones most likely to be picked up again.
func LocalBranches(ctx context.Context, run Runner, dir string) ([]string, error) {
	out, err := run(ctx, gitProgram, "-C", dir,
		"for-each-ref", "--format=%(refname:short)", "--sort=-committerdate", "refs/heads")
	if err != nil {
		return nil, fmt.Errorf("listing branches: %w", err)
	}

	var branches []string

	//nolint:modernize // SplitSeq returns a range-over-func iterator, which crashes gobco.
	for _, name := range strings.Split(text(out), "\n") {
		if name != "" && sanitize.Line(name) == name {
			branches = append(branches, name)
		}
	}

	return branches, nil
}

// Checkout switches to a branch. It carries nothing across: the interface
// refuses a dirty tree before calling this, leaving stashing to the person.
func Checkout(ctx context.Context, run Runner, dir, name string) error {
	_, err := run(ctx, gitProgram, "-C", dir, "switch", name)
	if err != nil {
		return fmt.Errorf("switching to %s: %w", name, err)
	}

	return nil
}

// WorktreeAdd creates branch name in a new worktree beside the repository and
// returns where it put it, so two tasks can be open at once — one checkout per
// worktree — where switching branches in place cannot. It starts the branch from
// start, or from HEAD when start is empty.
func WorktreeAdd(ctx context.Context, run Runner, dir, name, start string) (string, error) {
	path := worktreePath(dir, name)

	args := []string{"-C", dir, "worktree", "add"}
	if start != "" {
		// --no-track for the reason CreateBranch uses it: a branch off
		// origin/main should read as never pushed, not as ahead of it.
		args = append(args, "--no-track")
	}

	args = append(args, "-b", name, path)
	if start != "" {
		args = append(args, start)
	}

	_, err := run(ctx, gitProgram, args...)
	if err != nil {
		return "", fmt.Errorf("creating a worktree for %s: %w", name, err)
	}

	return path, nil
}

// worktreePath is where a worktree for a branch lives: a sibling of the
// repository named for the branch, out of its working tree but beside it.
func worktreePath(dir, name string) string {
	return dir + "-" + strings.ReplaceAll(name, "/", "-")
}

// FetchCommand updates the remote-tracking refs from origin, so a branch starts
// from what origin holds now rather than from whenever the user last fetched.
//
// Prompts are off, as they are for a push: a fetch can need a credential, and
// nobody can answer for one from inside the interface.
func FetchCommand(dir string) proc.Command {
	return proc.Command{
		Dir:  dir,
		Name: gitProgram,
		Args: []string{"fetch", "origin"},
		Env:  []string{noTerminalPrompt},
	}
}

// PushCommand pushes a branch to origin and makes it the upstream.
//
// Prompts are off: nobody can answer a credential prompt from inside the
// interface, so git must fail with a reason rather than wait forever for input
// that is never coming.
func PushCommand(dir, branch string) proc.Command {
	return proc.Command{
		Dir:  dir,
		Name: gitProgram,
		Args: []string{"push", "--set-upstream", "origin", branch},
		Env:  []string{noTerminalPrompt},
	}
}

// CommitCommand commits the index with the message in a file. It is a plain
// git commit, so the repository's hooks run exactly as they do in a terminal.
func CommitCommand(dir, messageFile string) proc.Command {
	return proc.Command{Dir: dir, Name: gitProgram, Args: []string{"commit", "--file", messageFile}, Env: nil}
}

// RebaseCommand replays the branch onto base, the branch's fetched merge target.
// A conflict stops git with a non-zero exit and leaves the repository mid-rebase
// for the shell to finish, which the interface cannot do.
func RebaseCommand(dir, base string) proc.Command {
	return proc.Command{
		Dir:  dir,
		Name: gitProgram,
		Args: []string{"rebase", base},
		Env:  []string{noTerminalPrompt},
	}
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
