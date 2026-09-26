// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// heldFacts are what a surface already holds about a merged pull request, each
// value distinct, so a field carried into the wrong place shows.
func heldFacts() loop.AnnouncementFacts {
	return loop.AnnouncementFacts{
		Author:   author,
		Pull:     forge.PullRequest{Number: 9, URL: pullURL, Title: subject},
		IssueKey: issueKey, IssueSummary: summary, IssueURL: browseURL,
		Moment: messaging.MomentMerged,
	}
}

func TestAnAnnouncementSaysWhatTheSurfaceHolds(t *testing.T) {
	t.Parallel()

	// Act
	got := loop.Announcement(heldFacts(), announceConfig(), forge.KindGitLab)

	// Assert
	want := messaging.Announcement{
		Author: author, PullRequestURL: pullURL, PullRequestTitle: subject,
		IssueKey: issueKey, IssueSummary: summary, IssueURL: browseURL,
		Noun: "merge request", Moment: messaging.MomentMerged, Kind: config.KindSlack, Template: template,
	}
	if got != want {
		t.Errorf("Announcement = %+v, want %+v", got, want)
	}
}

func TestComposingAnnouncesTheFactsItsSeamsRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// The seams answer with an open pull request whose CI passed, titled pullTitle.
	read := heldFacts()
	read.Pull, read.Moment = openPull(), messaging.MomentReady

	// Act
	composed, _, err := compose(announceSeams())

	// Assert
	want := loop.Announcement(read, announceConfig(), forge.KindGitLab)
	if err != nil || composed != want {
		t.Errorf("ComposeAnnouncement = %+v, %v; want %+v", composed, err, want)
	}
}
