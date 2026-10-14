// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// readBranchOnto reads the branch again onto the base the composer was
// answered with, so the title, the body and the reviewers are proposed from
// the commits that pull request would merge. A base left as it was drafted
// onto, or left empty, reads nothing.
func readBranchOnto(ctx context.Context, deps Deps, composer prComposer) tea.Cmd {
	read, base := deps.Git.BranchOnto, strings.TrimSpace(composer.answers.base)
	if read == nil || base == "" || base == composer.draftedOnto {
		return nil
	}

	opened := composer.opened

	return func() tea.Msg {
		branch, err := read(ctx, base)

		return branchOntoRead{opened: opened, base: base, branch: branch, err: err}
	}
}

// branchOntoRead is the branch read onto base for the composer opened as
// opened.
type branchOntoRead struct {
	opened int
	base   string
	branch gitrepo.Branch
	err    error
}

var _ applier = branchOntoRead{}

// apply drafts the composer again from the commits past the base, and reads
// the owners again for reviewers still untyped over. A base that could not be
// read, or that the branch holds nothing past, keeps the draft it has: the
// forge says what is wrong with it on open. Nothing changes once the composer
// has closed.
func (read branchOntoRead) apply(m Model) (Model, tea.Cmd) {
	composer, open := m.overlay.(prComposer)
	if !open || composer.opened != read.opened {
		return m, nil
	}

	composer.awaiting.onto = false

	var reviewers tea.Cmd
	if read.err == nil && len(read.branch.Commits) > 0 {
		composer, reviewers = composer.redrafted(m, read.base, read.branch.Commits)
	}

	m, opening := composer.settled(m)

	return m, tea.Batch(reviewers, opening)
}

// redrafted is the composer drafted again from the commits past base, and
// the read of the owners for reviewers still untyped over, which it then
// waits on.
func (c prComposer) redrafted(m Model, base string, commits []gitrepo.Commit) (prComposer, tea.Cmd) {
	c = c.draftedFrom(base, commits)
	if c.reviewersSettled && c.answers.reviewers != c.reviewersProposed {
		return c, nil
	}

	c.reviewersSettled = false
	reviewers := readReviewers(m.session, m.deps, c)
	c.awaiting.reviewers = reviewers != nil

	return c, reviewers
}

// draftedFrom proposes the title and body again from commits, the branch's past
// base: the title only while it is still the one proposed, the body only while
// it is unedited.
func (c prComposer) draftedFrom(base string, commits []gitrepo.Commit) prComposer {
	proposed, _ := loop.Draft(c.proposedFrom)

	c.proposedFrom.Subjects = loop.Subjects(commits)
	c.draftedOnto = base

	if c.answers.title == proposed {
		c.answers.title, _ = loop.Draft(c.proposedFrom)
	}

	if !c.edited {
		c = c.withTemplate(c.answers.template)
	}

	return c
}
