// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jacob-delgado/workflow/internal/cli/console"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// followUp makes the interface's two offers once the pull request is open: to
// link it on the branch's issue, then to move the issue to the review status,
// adding each offer made to report. A link that fails does not keep the move
// from being offered, and the command still fails with it; a question nothing
// can answer stops both.
func followUp(
	ctx context.Context, notes console.Notes, seams prSeams, opened openedPull, opts writeOptions, report prReport,
) (prReport, error) {
	link, linkErr := offerLink(ctx, notes, seams, opened, opts)
	report.FollowUps = append(report.FollowUps, link...)

	if errors.Is(linkErr, errNoTerminal) {
		return report, linkErr
	}

	move, moveErr := offerReviewStatus(ctx, notes, seams, opened.issueKey, opts)
	report.FollowUps = append(report.FollowUps, move...)

	return report, errors.Join(linkErr, moveErr)
}

// offerLink offers to add the just-opened pull request's link to the branch's
// issue, so the team that watches Jira sees it, and reports the offer made. A
// branch that names no Jira issue, or a tracker that cannot take a link — the
// forge's own issues — is offered nothing. A failed link is said at once,
// before the move is offered.
func offerLink(
	ctx context.Context, notes console.Notes, seams prSeams, opened openedPull, opts writeOptions,
) ([]followUpReport, error) {
	if seams.LinkPull == nil || opened.issueKey == "" {
		return nil, nil
	}

	pull, issue := seams.Kind.Sigil()+strconv.Itoa(opened.pull.Number), string(opened.issueKey)
	offer := followUpReport{Action: followUpLink, IssueKey: issue}

	proceed, err := opts.proceed(notes, seams.Confirm, writePrompt{
		question: "Link " + pull + " on " + issue + "?",
		dryRun:   "dry run: would link it on " + issue,
		declined: "Left " + issue + " unlinked.",
	})
	if err != nil || !proceed {
		return []followUpReport{offer}, err
	}

	err = seams.LinkPull(ctx, opened.issueKey, opened.pull.URL, opened.pull.Title)
	if err != nil {
		notes.Warn("Could not link " + pull + " on " + issue + ".")

		return []followUpReport{offer}, fmt.Errorf("linking %s on %s: %w", pull, issue, err)
	}

	notes.Note("Linked " + pull + " on " + issue + ".")

	offer.Done = true

	return []followUpReport{offer}, nil
}

// offerReviewStatus offers to move the branch's Jira issue to the configured
// review status once the pull request is open, chosen by name because it shares
// a category with "in progress", and reports the offer made. No Jira issue, a
// read that fails, a status Jira does not offer, or one whose transition needs
// fields this command cannot fill, is passed over quietly — the pull request is
// already open. Everything it says is commentary on the open, so it goes to
// notes.
func offerReviewStatus(
	ctx context.Context, notes console.Notes, seams prSeams, issueKey jira.Key, opts writeOptions,
) ([]followUpReport, error) {
	target, ok := loop.ReviewTransition(ctx, seams.Transitions, issueKey, seams.ReviewStatus)
	if !ok {
		return nil, nil
	}

	offer := followUpReport{Action: followUpTransition, IssueKey: string(issueKey), Status: target.ToStatus}

	proceed, err := opts.proceed(notes, seams.Confirm, writePrompt{
		question: "Move " + string(issueKey) + " to " + target.ToStatus + "?",
		dryRun:   "dry run: would move " + string(issueKey) + " to " + target.ToStatus,
		declined: "Left " + string(issueKey) + " as it is.",
	})
	if err != nil || !proceed {
		return []followUpReport{offer}, err
	}

	err = seams.Transition(ctx, issueKey, target, nil)
	if err != nil {
		return []followUpReport{offer}, fmt.Errorf("moving %s to %s: %w", issueKey, target.ToStatus, err)
	}

	notes.Note("Moved " + string(issueKey) + " to " + target.ToStatus + ".")

	offer.Done = true

	return []followUpReport{offer}, nil
}
