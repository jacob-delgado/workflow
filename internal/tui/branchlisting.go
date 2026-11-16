// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"slices"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// branchLister lists the local and remote branches, for the switcher to offer
// and the Issues list to mark, and asks the tracker which of the issues they
// name are yours. A nil remote lists none there; a nil search asks nothing, and
// a failed one answers nothing, so every issue counts.
type branchLister struct {
	local     func(ctx context.Context) ([]string, error)
	remote    func(ctx context.Context) ([]string, error)
	links     func(ctx context.Context) map[string]string
	search    func(ctx context.Context, jql string, startAt int) (jira.SearchResult, error)
	project   string
	worktrees func(ctx context.Context) ([]gitrepo.Worktree, error)
	// here is the root of the worktree this session works in.
	here string
}

// otherWorktrees is each other worktree, gone or not, by the branch it has
// checked out: none outside a repository or when the read fails, when each
// branch is checked out as before.
func (l branchLister) otherWorktrees(ctx context.Context) map[string]gitrepo.Worktree {
	byBranch := map[string]gitrepo.Worktree{}
	if l.worktrees == nil {
		return byBranch
	}

	worktrees, _ := l.worktrees(ctx)
	for _, worktree := range worktrees {
		if worktree.Branch != "" && worktree.Dir != l.here {
			byBranch[worktree.Branch] = worktree
		}
	}

	return byBranch
}

// list is the local branches, then the remote ones, then which issues are yours,
// asked under ctx. A listing git could not make is the listing's error; a
// tracker that could not be asked leaves every issue counted, with the reason
// it was not.
func (l branchLister) list(ctx context.Context) branchesListed {
	local, err := l.local(ctx)
	if err != nil {
		return branchesListed{err: err}
	}

	remote, err := l.remoteNames(ctx)
	if err != nil {
		return branchesListed{err: err}
	}

	links := l.linked(ctx)
	if l.search == nil {
		return branchesListed{local: local, remote: remote, links: links}
	}

	mine, err := loop.AssignedKeys(ctx, l.search, l.issueKeys(slices.Concat(local, remote), links))
	if err != nil {
		return branchesListed{local: local, remote: remote, links: links, notAsked: err}
	}

	return branchesListed{local: local, remote: remote, links: links, mine: mine}
}

// linked is every branch linked to an issue by hand, or none with no
// repository to ask.
func (l branchLister) linked(ctx context.Context) map[string]string {
	if l.links == nil {
		return nil
	}

	return l.links(ctx)
}

// remoteNames lists the remote's branches, or none with no repository to ask.
func (l branchLister) remoteNames(ctx context.Context) ([]string, error) {
	if l.remote == nil {
		return nil, nil
	}

	return l.remote(ctx)
}

// issueKeys is the issue each of names is for, by its link in links or its
// name, for those that are for one.
func (l branchLister) issueKeys(names []string, links map[string]string) []jira.Key {
	var keys []jira.Key

	for _, name := range names {
		if key, named := loop.NamedIssue(name, links, l.project); named {
			keys = append(keys, jira.Key(key.Key))
		}
	}

	return keys
}
