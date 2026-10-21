// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"slices"
	"time"

	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// The repository's remote and the branch work merges into.
const (
	remote   = gitrepo.DefaultRemote
	mainName = "main"
	baseRef  = remote + "/" + mainName
	// mainHead is the commit main and origin/main stand at.
	mainHead = "300a7be"
)

// repository is the demo's checkout: the branches, the one checked out, and
// the changes in its work tree.
type repository struct {
	current  string
	branches map[string]localBranch
	// order is the branches most recently committed to first, as LocalBranches
	// lists them; branches that stand at the same commit go by name, as
	// for-each-ref breaks the tie.
	order   []string
	changes []fileChange
	// diffs are each changed file's diff against HEAD, by its path.
	diffs map[string][]string
	// written are the commits made since the demo began, when each was made,
	// for the Summary.
	written []gitrepo.DatedCommit
}

// localBranch is a branch's own history: its commits past main, how many of
// them its upstream holds, and the upstream, empty until it is pushed.
type localBranch struct {
	upstream string
	commits  []gitrepo.Commit
	pushed   int
}

// fileChange is a file the morning's work changed, as git status shows it
// either side of staging.
type fileChange struct {
	staged       bool
	asStaged, as gitrepo.Change
}

// newRepository is api checked out on main, up to date with origin, with the
// morning's redaction work in the work tree; the two other issues in flight
// have branches of their own, as the web mock's do (mockSnapshot.ts).
func newRepository() repository {
	return repository{
		current: mainName,
		branches: map[string]localBranch{
			mainName:                        {upstream: baseRef},
			"feat/PROJ-418-webhook-retries": {},
			"fix/PROJ-408-flaky-timeout":    {},
		},
		order:   []string{"feat/PROJ-418-webhook-retries", "fix/PROJ-408-flaky-timeout", mainName},
		changes: morningChanges(),
		diffs:   morningDiffs(),
	}
}

// The redaction's files.
const (
	reqlogPath     = "internal/wiring/reqlog.go"
	redactPath     = "internal/config/redact.go"
	reqlogTestPath = "internal/wiring/reqlog_test.go"
)

// morningChanges are the redaction's files: two modified, one new.
func morningChanges() []fileChange {
	return []fileChange{modified(reqlogPath), modified(redactPath), untracked(reqlogTestPath)}
}

// morningDiffs are the redaction's diffs, by file.
func morningDiffs() map[string][]string {
	return map[string][]string{
		reqlogPath: {
			"@@ -44 +44 @@ func (l reqLog) write(r *http.Request) {",
			"-\tl.out.Printf(\"%s %v\", r.URL, r.Header)",
			"+\tl.out.Printf(\"%s %v\", r.URL, config.Redact(r.Header))",
		},
		redactPath: {
			"@@ -14 +14,3 @@ var sensitive = []string{",
			" \t\"Authorization\",",
			"+\t\"Proxy-Authorization\",",
			"+\t\"X-Api-Key\",",
		},
		reqlogTestPath: {
			"@@ -0,0 +1,3 @@",
			"+func TestTheRequestLogCarriesNoToken(t *testing.T) {",
			"+\tt.Parallel()",
			"+}",
		},
	}
}

// modified is a tracked file changed in the work tree.
func modified(path string) fileChange {
	return fileChange{
		as:       gitrepo.Change{Path: path, Staged: ' ', Unstaged: 'M'},
		asStaged: gitrepo.Change{Path: path, Staged: 'M', Unstaged: ' '},
	}
}

// untracked is a file git does not track yet.
func untracked(path string) fileChange {
	return fileChange{
		as:       gitrepo.Change{Path: path, Staged: '?', Unstaged: '?'},
		asStaged: gitrepo.Change{Path: path, Staged: 'A', Unstaged: ' '},
	}
}

// shown is the change as git status shows it now.
func (c fileChange) shown() gitrepo.Change {
	if c.staged {
		return c.asStaged
	}

	return c.as
}

// gitDeps is the demo's git.
func (w *World) gitDeps() seams.Git {
	return seams.Git{
		Branch:   w.branch,
		Changes:  w.changes,
		Diff:     w.diff,
		Branches: w.branches,
		// The Summary's commits.
		CommitsBetween: w.commitsBetween,
		// The code owners of the branch's changes, which a pull request
		// proposes as its reviewers.
		ChangedPaths: w.changedPaths,
		CodeOwnersAt: codeOwnersAt,
		// What the demo does to the repository.
		RemoteBranches: w.remoteBranches,
		CreateBranch:   w.createBranch,
		Stage:          w.stage,
		Unstage:        w.unstage,
		Commit:         w.commit,
		Push:           w.push,
	}
}

// apiCodeOwners is api's CODEOWNERS: the api reviewers own everything, Sam
// the configuration, and Lee with the platform team the wiring.
const apiCodeOwners = "* @acme/api-reviewers\n/internal/config/ @sam.ortiz\n" +
	"/internal/wiring/ @lee.chen @acme/platform\n"

// codeOwnersAt is api's CODEOWNERS, at any ref.
func codeOwnersAt(context.Context, string) (codeowners.File, bool, error) {
	return codeowners.Parse(apiCodeOwners, codeowners.GitHub), true, nil
}

// changedPaths is the redaction's files once the branch checked out has a
// commit past main, and none before.
func (w *World) changedPaths(context.Context, string) ([]string, error) {
	committed := locked(w, func() bool { return len(w.repo.branches[w.repo.current].commits) > 0 })
	if !committed {
		return nil, nil
	}

	return []string{reqlogPath, redactPath, reqlogTestPath}, nil
}

// branch is where the checked-out branch stands against main.
func (w *World) branch(context.Context) (gitrepo.Branch, error) {
	return locked(w, w.repo.checkedOut), nil
}

// checkedOut is the branch checked out, as git reads it.
func (r repository) checkedOut() gitrepo.Branch {
	local := r.branches[r.current]

	// Git counts a branch ahead of its upstream, so one never pushed is
	// ahead of nothing.
	ahead := len(local.commits) - local.pushed
	if local.upstream == "" {
		ahead = 0
	}

	return gitrepo.Branch{
		Name: r.current, Head: headOf(local), Upstream: local.upstream, PushRemote: remote, BaseRemote: remote,
		Ahead: ahead, Base: baseRef, BaseUpdated: epoch().Add(-3 * time.Hour),
		Commits: slices.Clone(local.commits),
	}
}

// changes is the work tree's changes, staged or not.
func (w *World) changes(context.Context) ([]gitrepo.Change, error) {
	return locked(w, func() []gitrepo.Change {
		shown := make([]gitrepo.Change, 0, len(w.repo.changes))
		for _, change := range w.repo.changes {
			shown = append(shown, change.shown())
		}

		return shown
	}), nil
}

// diff is a changed file's diff against HEAD.
func (w *World) diff(_ context.Context, change gitrepo.Change) ([]string, error) {
	return locked(w, func() []string { return slices.Clone(w.repo.diffs[change.Path]) }), nil
}

// branches is the local branches, most recently committed to first.
func (w *World) branches(context.Context) ([]string, error) {
	return locked(w, func() []string { return slices.Clone(w.repo.order) }), nil
}
