// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"errors"
	"fmt"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// ErrNoPullRequest refuses an announcement when the branch has no pull request
// to announce, or there is no way to find one.
var ErrNoPullRequest = errors.New("there is no pull request to announce")

// AnnounceSeams are what composing an announcement reads. A nil Branch or
// FindPull means there is nothing to announce; a nil Author, Issue, BrowseURL
// or CheckCI only makes the announcement say less.
type AnnounceSeams struct {
	Branch    func() (gitrepo.Branch, error)
	FindPull  func(branch string) (forge.PullRequest, bool, error)
	Author    func() (string, error)
	Issue     func(jira.Key) (jira.IssueDetail, error)
	BrowseURL func(jira.Key) string
	CheckCI   func(pull forge.PullRequest, head string) (forge.CI, error)
}

// AnnounceMoment is the moment a pull request is at: merged, its CI red, or —
// the common case — open and ready for review. ci is what its CI last said; the
// zero CI, which reports none, reads as ready, so a caller that could not read
// CI passes it.
func AnnounceMoment(pull forge.PullRequest, ci forge.CI) messaging.Moment {
	switch {
	case pull.State == forge.StateMerged:
		return messaging.MomentMerged
	case ci.State == forge.CIFailed:
		return messaging.MomentCIRed
	default:
		return messaging.MomentReady
	}
}

// ComposeAnnouncement builds the announcement for the checked-out branch's pull
// request — its title and link, the branch's issue, who opened it and the moment
// it is at — rendered for the configured service and template, and returns the
// pull request it announces. It refuses with ErrNoPullRequest when there is none,
// and says which read failed when the branch or the pull request cannot be read.
func ComposeAnnouncement(
	seams AnnounceSeams, cfg config.Messaging, project string, kind forge.Kind,
) (messaging.Announcement, forge.PullRequest, error) {
	branch, pull, err := pullToAnnounce(seams)
	if err != nil {
		return messaging.Announcement{}, forge.PullRequest{}, err
	}

	key, _ := convention.IssueKey(branch.Name, project)
	issueKey := jira.Key(key)

	return messaging.Announcement{
		Author:           authorName(seams.Author),
		PullRequestURL:   pull.URL,
		PullRequestTitle: pull.Title,
		IssueKey:         key,
		IssueSummary:     issueSummary(seams.Issue, issueKey),
		IssueURL:         issueURL(seams.BrowseURL, issueKey),
		Noun:             kind.Noun(),
		Moment:           momentOf(seams.CheckCI, pull, branch.Head),
		Kind:             cfg.Kind,
		Template:         cfg.Announcement,
	}, pull, nil
}

// pullToAnnounce reads the checked-out branch and the pull request found for it,
// open or merged.
func pullToAnnounce(seams AnnounceSeams) (gitrepo.Branch, forge.PullRequest, error) {
	if seams.Branch == nil || seams.FindPull == nil {
		return gitrepo.Branch{}, forge.PullRequest{}, ErrNoPullRequest
	}

	branch, err := seams.Branch()
	if err != nil {
		return gitrepo.Branch{}, forge.PullRequest{}, fmt.Errorf("reading the branch: %w", err)
	}

	pull, found, err := seams.FindPull(branch.Name)
	if err != nil {
		return gitrepo.Branch{}, forge.PullRequest{}, fmt.Errorf("reading the pull request: %w", err)
	}

	if !found {
		return gitrepo.Branch{}, forge.PullRequest{}, ErrNoPullRequest
	}

	return branch, pull, nil
}

// momentOf is the pull request's moment, reading its CI only when the moment
// could turn on it — never for a merged pull request, which the forge would be
// asked about for nothing. A CI that cannot be read leaves the moment at ready
// rather than failing the announcement.
func momentOf(
	check func(forge.PullRequest, string) (forge.CI, error), pull forge.PullRequest, head string,
) messaging.Moment {
	if pull.State == forge.StateMerged || check == nil {
		return AnnounceMoment(pull, forge.CI{})
	}

	ci, err := check(pull, head)
	if err != nil {
		return AnnounceMoment(pull, forge.CI{})
	}

	return AnnounceMoment(pull, ci)
}

// authorName is who the forge credential belongs to, or empty when the forge
// will not say — the announcement reads without it.
func authorName(read func() (string, error)) string {
	if read == nil {
		return ""
	}

	name, err := read()
	if err != nil {
		return ""
	}

	return name
}
