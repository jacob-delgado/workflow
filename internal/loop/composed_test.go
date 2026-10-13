// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"context"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestComposeAnnouncementAnswersTheBranchItRead(t *testing.T) {
	t.Parallel()

	// Act
	composed, err := loop.ComposeAnnouncement(t.Context(), announceSeams(), announceConfig(), project, forge.KindGitLab)

	// Assert
	if err != nil || composed.Branch.Head != headCommit || composed.Pull.Number != 9 {
		t.Errorf("ComposeAnnouncement = branch head %q, pull %d (%v); want head %q, pull 9",
			composed.Branch.Head, composed.Pull.Number, err, headCommit)
	}
}

func TestComposeAnnouncementOfComposesForAPullRequestJustOpened(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := announceSeams()
	seams.FindPull = func(context.Context, string) (forge.PullRequest, bool, error) {
		t.Error("FindPull was asked of a pull request already in hand")

		return forge.PullRequest{}, false, nil
	}

	// Act
	announcement := loop.ComposeAnnouncementOf(t.Context(), seams, openable(branchName), openPull(),
		announceConfig(), project, forge.KindGitLab)

	// Assert
	if announcement.PullRequestURL != pullURL || announcement.IssueKey != issueKey ||
		announcement.Moment != messaging.MomentReady {
		t.Errorf("ComposeAnnouncementOf = %+v; want the ready announcement of %s for %s", announcement, pullURL, issueKey)
	}
}
