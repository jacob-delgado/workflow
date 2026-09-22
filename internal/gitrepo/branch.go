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

// gitProgram is the program every command here runs, commitVerb the subcommand
// its commit builders share.
const (
	gitProgram = "git"
	commitVerb = "commit"
)

// DefaultRemote is the remote this package fetches from and reads base branches
// against, written once here rather than spelled out at each use. A push can go
// elsewhere: PushRemote consults remote.pushDefault and falls back to this.
const DefaultRemote = "origin"

// remoteBranch is a branch on DefaultRemote, "origin/main".
func remoteBranch(name string) string {
	return DefaultRemote + "/" + name
}

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
	// Truncated is set when the branch has more commits than are kept, so a
	// reader knows Commits is the oldest of a longer history, not the whole of it.
	Truncated bool
}

// Pushed reports whether origin holds this branch, under its own name, with
// every commit. The name matters: a branch created from origin/main without
// --no-track tracks origin/main, is ahead of it by nothing, and is not on the
// remote at all.
func (b Branch) Pushed() bool {
	return b.Name != "" && b.Upstream == remoteBranch(b.Name) && b.Ahead == 0
}

// Unpushed are the commits on this branch that have not reached the upstream,
// oldest first, so amending or fixing one up rewrites only local history. With
// no upstream every commit is unpushed; otherwise it is the last Ahead of them.
//
// A branch long enough to be truncated returns none: Commits then keeps the
// oldest, while Ahead is counted against the full range, so the last Ahead of
// the retained commits are the wrong ones — some already pushed. Rather than
// risk offering pushed history to rewrite, a truncated branch offers nothing.
func (b Branch) Unpushed() []Commit {
	if b.Truncated {
		return nil
	}

	if b.Upstream == "" {
		return b.Commits
	}

	return b.Commits[max(0, len(b.Commits)-b.Ahead):]
}

// HasUnpushedWork reports commits on this branch that origin is known not to
// hold: the branch has an upstream and is ahead of it. It is what guards a force
// delete — a merged branch normally has none, but one committed to since it was
// pushed does, and those commits would be lost. With no upstream to compare
// against nothing is known to be unpushed, so it reports false.
func (b Branch) HasUnpushedWork() bool {
	return b.Upstream != "" && b.Ahead > 0
}

// BaseName is the base branch without its remote prefix — "main" for
// "origin/main" — which is the name to show and to rebase onto. A base with no
// remote (a local "main") is returned as it is.
func (b Branch) BaseName() string {
	if _, name, found := strings.Cut(b.Base, "/"); found {
		return name
	}

	return b.Base
}

// ReadBranch reads where the checked-out branch stands. Only failing to read the
// branch at all is an error: no commits, no upstream and no base are ordinary
// states for a repository to be in, and each is left empty.
func (r Repository) ReadBranch(ctx context.Context) (Branch, error) {
	name, err := r.run(ctx, "git", "-C", r.dir, "branch", "--show-current")
	if err != nil {
		return Branch{}, readFailure(ctx, r.run, r.dir, "reading the current branch of "+r.dir, err)
	}

	git := func(args ...string) string {
		return optional(ctx, r.run, append([]string{"-C", r.dir}, args...)...)
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

		branch.Truncated = len(commits) > commitLimit
		if branch.Truncated {
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
	if origin := git("symbolic-ref", "--quiet", "--short", "refs/remotes/"+DefaultRemote+"/HEAD"); origin != "" {
		return origin
	}

	candidates := []struct{ ref, name string }{
		{ref: "refs/remotes/" + remoteBranch("main"), name: remoteBranch("main")},
		{ref: "refs/remotes/" + remoteBranch("master"), name: remoteBranch("master")},
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
func (r Repository) CreateBranch(ctx context.Context, name, start string) error {
	args := []string{"-C", r.dir, "switch", "--create", name}
	if start != "" {
		args = append(args, "--no-track", start)
	}

	_, err := r.run(ctx, "git", args...)
	if err != nil {
		return fmt.Errorf("creating branch %s: %w", name, err)
	}

	return nil
}

// LocalBranches lists the repository's local branches, most recently committed
// to first, so a switcher offers the ones most likely to be picked up again.
func (r Repository) LocalBranches(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, gitProgram, "-C", r.dir,
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

// RemoteBranches lists the branches that exist on the remotes, by name and most
// recently committed to first, so the base field can complete to one. The remote
// prefix is dropped ("origin/main" becomes "main") to match the name a base
// carries, a remote's symbolic HEAD pointer is left out (git abbreviates it to
// the bare remote name, "origin", which has no branch part), and a branch on
// more than one remote is listed once.
func (r Repository) RemoteBranches(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, gitProgram, "-C", r.dir,
		"for-each-ref", "--format=%(refname:short)", "--sort=-committerdate", "refs/remotes")
	if err != nil {
		return nil, fmt.Errorf("listing remote branches: %w", err)
	}

	var branches []string

	seen := map[string]bool{}

	//nolint:modernize // SplitSeq returns a range-over-func iterator, which crashes gobco.
	for _, ref := range strings.Split(text(out), "\n") {
		_, name, found := strings.Cut(ref, "/")
		if !found || name == "" || name == "HEAD" {
			continue
		}

		if seen[name] || sanitize.Line(name) != name {
			continue
		}

		seen[name] = true
		branches = append(branches, name)
	}

	return branches, nil
}

// Checkout switches to a branch. It carries nothing across: the interface
// refuses a dirty tree before calling this, leaving stashing to the person.
//
// The name can come straight from the web request body, so "--" ends git's
// option parsing: a name beginning with a dash is then read as a ref, never as
// an option such as --orphan.
func (r Repository) Checkout(ctx context.Context, name string) error {
	_, err := r.run(ctx, gitProgram, "-C", r.dir, "switch", "--", name)
	if err != nil {
		return fmt.Errorf("switching to %s: %w", name, err)
	}

	return nil
}

// FinishBranch finishes a merged branch: switch to base, fast-forward it, and
// delete the branch. It is for after the branch's pull request has merged, so
// the local repository catches up and the branch is cleaned away.
//
// The delete is a force delete (-D) on purpose: a squash or rebase merge leaves
// the branch's own commits unreachable from base, so the safe -d would refuse a
// branch the forge has already merged. Its caller is what makes this safe: it
// offers the finish only for a merged branch with no unpushed commits (see
// Branch.HasUnpushedWork), so the commits -D discards are the ones origin and
// the merge already hold.
func (r Repository) FinishBranch(ctx context.Context, branch, base string) error {
	steps := [][]string{
		{"switch", base},
		{"pull", "--ff-only"},
		{"branch", "-D", branch},
	}

	for _, step := range steps {
		_, err := r.run(ctx, gitProgram, append([]string{"-C", r.dir}, step...)...)
		if err != nil {
			return fmt.Errorf("git %s: %w", strings.Join(step, " "), err)
		}
	}

	return nil
}

// WorktreeAdd creates branch name in a new worktree beside the repository and
// returns where it put it, so two tasks can be open at once — one checkout per
// worktree — where switching branches in place cannot. It starts the branch from
// start, or from HEAD when start is empty.
func (r Repository) WorktreeAdd(ctx context.Context, name, start string) (string, error) {
	path := worktreePath(r.dir, name)

	args := []string{"-C", r.dir, "worktree", "add"}
	if start != "" {
		// --no-track for the reason CreateBranch uses it: a branch off
		// origin/main should read as never pushed, not as ahead of it.
		args = append(args, "--no-track")
	}

	args = append(args, "-b", name, path)
	if start != "" {
		args = append(args, start)
	}

	_, err := r.run(ctx, gitProgram, args...)
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
		Args: []string{"fetch", DefaultRemote},
		Env:  []string{noTerminalPrompt},
	}
}

// PushCommand pushes a branch to remote and makes it the upstream.
//
// Prompts are off: nobody can answer a credential prompt from inside the
// interface, so git must fail with a reason rather than wait forever for input
// that is never coming.
func PushCommand(dir, remote, branch string) proc.Command {
	return proc.Command{
		Dir:  dir,
		Name: gitProgram,
		Args: []string{"push", "--set-upstream", remote, branch},
		Env:  []string{noTerminalPrompt},
	}
}

// CommitCommand commits the index with the message in a file. It is a plain
// git commit, so the repository's hooks run exactly as they do in a terminal.
func CommitCommand(dir, messageFile string) proc.Command {
	return proc.Command{Dir: dir, Name: gitProgram, Args: []string{commitVerb, "--file", messageFile}, Env: nil}
}

// AmendCommand folds the index into the last commit, keeping its message. Env is
// nil, like CommitCommand, so the repository's hooks run.
func AmendCommand(dir string) proc.Command {
	return proc.Command{Dir: dir, Name: gitProgram, Args: []string{commitVerb, "--amend", "--no-edit"}, Env: nil}
}

// FixupCommand records a fixup! commit of hash — a commit git squashes into that
// one on the next autosquash rebase. Env is nil so the repository's hooks run.
func FixupCommand(dir, hash string) proc.Command {
	return proc.Command{Dir: dir, Name: gitProgram, Args: []string{commitVerb, "--fixup=" + hash}, Env: nil}
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
func (r Repository) HooksDir(ctx context.Context) (string, error) {
	out, err := r.run(ctx, "git", "-C", r.dir, "rev-parse", "--git-path", "hooks")
	if err != nil {
		return "", fmt.Errorf("finding the hooks directory of %s: %w", r.dir, err)
	}

	path := text(out)
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.dir, path)
	}

	return path, nil
}

// PushRemote is the remote a push goes to: remote.pushDefault when the
// repository sets one — a fork pushes to its own remote, not origin — or
// DefaultRemote otherwise. git exits non-zero when the key is unset, which
// optional reads as the empty string, so an unset default falls back cleanly.
func (r Repository) PushRemote(ctx context.Context) string {
	if remote := optional(ctx, r.run, "-C", r.dir, "config", "--get", "remote.pushDefault"); remote != "" {
		return remote
	}

	return DefaultRemote
}
