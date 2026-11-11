// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"context"
	"errors"
	"fmt"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/progress"
)

var (
	// ErrNoPullRequest refuses an announcement when the branch has no pull
	// request to announce, or there is no way to find one.
	ErrNoPullRequest = errors.New("there is no pull request to announce")
	// ErrEmptyAnnouncement refuses an announcement edited down to nothing:
	// guidance, not a failure.
	ErrEmptyAnnouncement = errors.New("nothing to announce: the message was empty")
)

// AnnounceSeams are what composing an announcement reads. A nil Branch or
// FindPull means there is nothing to announce; a nil Author, Issue or
// BrowseURL only makes the announcement say less.
type AnnounceSeams struct {
	Branch    func(ctx context.Context) (gitrepo.Branch, error)
	FindPull  func(ctx context.Context, branch string) (forge.PullRequest, bool, error)
	Author    func(ctx context.Context) (string, error)
	Issue     func(ctx context.Context, issueKey jira.Key) (jira.IssueDetail, error)
	BrowseURL func(jira.Key) string
}

// AnnounceMoment is the moment a pull request is at: merged, or — the common
// case — open and ready for review. Its CI decides whether it may be
// announced (see ClearToAnnounce), never what is said of it.
func AnnounceMoment(pull forge.PullRequest) messaging.Moment {
	if pull.State == forge.StateMerged {
		return messaging.MomentMerged
	}

	return messaging.MomentReady
}

// Composed is an announcement composed for the checked-out branch's pull
// request, the pull request it announces, and the branch it was read from,
// whose head is the commit the pull request's CI is read at.
type Composed struct {
	Announcement messaging.Announcement
	Pull         forge.PullRequest
	Branch       gitrepo.Branch
}

// ComposeAnnouncement builds the announcement for the checked-out branch's pull
// request — its title and link, the branch's issue, who opened it and the moment
// it is at — rendered for the configured service and template, with the pull
// request it announces and the branch it read, reading under ctx. It refuses
// with ErrNoPullRequest when there is none, and says which read failed when the
// branch or the pull request cannot be read.
func ComposeAnnouncement(
	ctx context.Context, seams AnnounceSeams, cfg config.Messaging, project string, kind forge.Kind,
) (Composed, error) {
	branch, pull, err := pullToAnnounce(ctx, seams)
	if err != nil {
		return Composed{}, err
	}

	return Composed{
		Announcement: ComposeAnnouncementOf(ctx, seams, branch, pull, cfg, project, kind),
		Pull:         pull,
		Branch:       branch,
	}, nil
}

// ComposeAnnouncementOf builds the announcement of pull, opened from branch,
// as ComposeAnnouncement does, without asking the forge to find it: for a pull
// request just opened, which a fresh lookup might not find yet. It reads the
// author and the issue under ctx, and says less where they cannot be read.
func ComposeAnnouncementOf(
	ctx context.Context, seams AnnounceSeams, branch gitrepo.Branch, pull forge.PullRequest,
	cfg config.Messaging, project string, kind forge.Kind,
) messaging.Announcement {
	key, _, _ := BranchIssue(IssueSource{Branch: branch.Name, Link: branch.IssueLink, Pull: &pull, Project: project})
	issueKey := jira.Key(key.Key)

	return Announcement(AnnouncementFacts{
		Author:       authorName(ctx, seams.Author),
		Pull:         pull,
		IssueKey:     issueKey,
		IssueSummary: issueSummary(ctx, seams.Issue, issueKey),
		IssueURL:     issueURL(seams.BrowseURL, issueKey),
		Moment:       AnnounceMoment(pull),
	}, cfg, kind)
}

// AnnouncementFacts are what an announcement tells: who opened the pull
// request, the pull request, the issue its branch names — empty fields where
// there is none, or the tracker could not say — and the moment it marks.
type AnnouncementFacts struct {
	Author       string
	Pull         forge.PullRequest
	IssueKey     jira.Key
	IssueSummary string
	IssueURL     string
	Moment       messaging.Moment
}

// Announcement is the announcement of facts a surface already holds, rendered
// for the configured service and template, and calling the change what the
// forge of kind calls it. ComposeAnnouncement reads the facts through seams; a
// surface that holds them already, as the terminal does, passes them here.
func Announcement(facts AnnouncementFacts, cfg config.Messaging, kind forge.Kind) messaging.Announcement {
	return messaging.Announcement{
		Author:           facts.Author,
		PullRequestURL:   facts.Pull.URL,
		PullRequestTitle: facts.Pull.Title,
		IssueKey:         string(facts.IssueKey),
		IssueSummary:     facts.IssueSummary,
		IssueURL:         facts.IssueURL,
		Noun:             kind.Noun(),
		Moment:           facts.Moment,
		Kind:             cfg.Kind,
		Template:         cfg.Announcement,
	}
}

// pullToAnnounce reads the checked-out branch and the pull request found for it,
// open or merged: one closed without merging is none to announce.
func pullToAnnounce(ctx context.Context, seams AnnounceSeams) (gitrepo.Branch, forge.PullRequest, error) {
	if seams.Branch == nil || seams.FindPull == nil {
		return gitrepo.Branch{}, forge.PullRequest{}, ErrNoPullRequest
	}

	branch, err := seams.Branch(ctx)
	if err != nil {
		return gitrepo.Branch{}, forge.PullRequest{}, fmt.Errorf("reading the branch: %w", err)
	}

	pull, found, err := seams.FindPull(ctx, branch.Name)
	if err != nil {
		return gitrepo.Branch{}, forge.PullRequest{}, fmt.Errorf("reading the pull request: %w", err)
	}

	if !followed(pull, found) {
		return gitrepo.Branch{}, forge.PullRequest{}, ErrNoPullRequest
	}

	return branch, pull, nil
}

// followed reports that a find answered a pull request the loop follows: one
// found, open or merged. progress reads one closed without merging as none —
// its review not started, the next step to open one again — so it is not one
// to announce, nor one announced: the pull request opened next is announced
// afresh.
func followed(pull forge.PullRequest, found bool) bool {
	return found && progress.PullStateOf(pull.State) != progress.NoPullRequest
}

// authorName is who the forge credential belongs to, or empty when the forge
// will not say — the announcement reads without it.
func authorName(ctx context.Context, read func(context.Context) (string, error)) string {
	if read == nil {
		return ""
	}

	name, err := read(ctx)
	if err != nil {
		return ""
	}

	return name
}
