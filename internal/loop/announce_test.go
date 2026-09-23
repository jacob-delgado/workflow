// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// The pull request and its author these tests announce.
const (
	pullURL   = "https://gitlab.example.com/o/r/-/merge_requests/9"
	pullTitle = "Redact the token"
	author    = "Ada"
	template  = "{author} opened {noun} {url}"
)

// openPull is the branch's open pull request.
func openPull() forge.PullRequest {
	return forge.PullRequest{Number: 9, URL: pullURL, Title: pullTitle}
}

// announceSeams answer as a branch with an open pull request whose CI passed,
// an author the forge names, and an issue the tracker can read and link.
func announceSeams() loop.AnnounceSeams {
	return loop.AnnounceSeams{
		Branch:   func() (gitrepo.Branch, error) { return openable(branchName), nil },
		FindPull: func(string) (forge.PullRequest, bool, error) { return openPull(), true, nil },
		Author:   func() (string, error) { return author, nil },
		Issue: func(jira.Key) (jira.IssueDetail, error) {
			return jira.IssueDetail{Issue: jira.Issue{Key: issueKey, Summary: summary}}, nil
		},
		BrowseURL: func(key jira.Key) string { return "https://jira.example.com/browse/" + string(key) },
		CheckCI: func(forge.PullRequest, string) (forge.CI, error) {
			return forge.CI{State: forge.CIPassed}, nil
		},
	}
}

// announceConfig renders for Slack, with a template of the team's own.
func announceConfig() config.Messaging {
	return config.Messaging{Kind: config.MessagingKind("slack"), Channel: "#dev", Announcement: template}
}

// compose composes over seams for the tracker's project on GitLab.
func compose(seams loop.AnnounceSeams) (messaging.Announcement, forge.PullRequest, error) {
	return loop.ComposeAnnouncement(seams, announceConfig(), project, forge.KindGitLab)
}

func TestAnnounceMomentReadsMergedBeforeCI(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		pull forge.PullRequest
		ci   forge.CI
		want messaging.Moment
	}{
		"a merged pull, whatever its CI": {
			pull: forge.PullRequest{State: forge.StateMerged}, ci: forge.CI{State: forge.CIFailed},
			want: messaging.MomentMerged,
		},
		"an open pull whose CI failed": {ci: forge.CI{State: forge.CIFailed}, want: messaging.MomentCIRed},
		"an open pull whose CI passed": {ci: forge.CI{State: forge.CIPassed}, want: messaging.MomentReady},
		"an open pull with no CI read": {want: messaging.MomentReady},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := loop.AnnounceMoment(tt.pull, tt.ci); got != tt.want {
				t.Errorf("AnnounceMoment = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComposeAnnouncementComposesThePullRequest(t *testing.T) {
	t.Parallel()

	// Act
	announcement, pull, err := compose(announceSeams())

	// Assert
	if err != nil || pull.Number != 9 {
		t.Fatalf("ComposeAnnouncement = pull %d, %v; want pull 9", pull.Number, err)
	}

	want := messaging.Announcement{
		Author: author, PullRequestURL: pullURL, PullRequestTitle: pullTitle,
		IssueKey: issueKey, IssueSummary: summary, IssueURL: browseURL, Noun: "merge request",
		Moment: messaging.MomentReady, Kind: config.MessagingKind("slack"), Template: template,
	}
	if announcement != want {
		t.Errorf("announcement = %+v, want %+v", announcement, want)
	}
}

func TestComposeAnnouncementRefusesWithoutAPullRequest(t *testing.T) {
	t.Parallel()

	cases := map[string]func(loop.AnnounceSeams) loop.AnnounceSeams{
		"no way to read the branch": func(seams loop.AnnounceSeams) loop.AnnounceSeams {
			seams.Branch = nil

			return seams
		},
		"no way to ask the forge": func(seams loop.AnnounceSeams) loop.AnnounceSeams {
			seams.FindPull = nil

			return seams
		},
		"a branch with no pull request": func(seams loop.AnnounceSeams) loop.AnnounceSeams {
			seams.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, nil }

			return seams
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, _, err := compose(mutate(announceSeams()))

			// Assert
			if !errors.Is(err, loop.ErrNoPullRequest) {
				t.Errorf("ComposeAnnouncement returned %v, want ErrNoPullRequest", err)
			}
		})
	}
}

func TestComposeAnnouncementReportsWhatCannotBeRead(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		mutate func(loop.AnnounceSeams) loop.AnnounceSeams
		names  string
	}{
		"the branch": {
			mutate: func(seams loop.AnnounceSeams) loop.AnnounceSeams {
				seams.Branch = func() (gitrepo.Branch, error) { return gitrepo.Branch{}, errSeam }

				return seams
			},
			names: "reading the branch",
		},
		"the pull request": {
			mutate: func(seams loop.AnnounceSeams) loop.AnnounceSeams {
				seams.FindPull = func(string) (forge.PullRequest, bool, error) { return forge.PullRequest{}, false, errSeam }

				return seams
			},
			names: "reading the pull request",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, _, err := compose(tt.mutate(announceSeams()))

			// Assert
			if !errors.Is(err, errSeam) || errors.Is(err, loop.ErrNoPullRequest) || !strings.Contains(err.Error(), tt.names) {
				t.Errorf("ComposeAnnouncement returned %v, want the read's own failure naming %q", err, tt.names)
			}
		})
	}
}

func TestComposeAnnouncementNeverReadsCIForAMergedPull(t *testing.T) {
	t.Parallel()

	// Arrange
	checked := false

	seams := announceSeams()
	seams.FindPull = func(string) (forge.PullRequest, bool, error) {
		merged := openPull()
		merged.State = forge.StateMerged

		return merged, true, nil
	}
	seams.CheckCI = func(forge.PullRequest, string) (forge.CI, error) {
		checked = true

		return forge.CI{State: forge.CIFailed}, nil
	}

	// Act
	announcement, _, err := compose(seams)

	// Assert
	if err != nil || announcement.Moment != messaging.MomentMerged || checked {
		t.Errorf("moment = %v (%v), CI read = %t; want merged without reading CI", announcement.Moment, err, checked)
	}
}

func TestComposeAnnouncementMarksRedCI(t *testing.T) {
	t.Parallel()

	// Arrange
	var askedHead string

	seams := announceSeams()
	seams.CheckCI = func(_ forge.PullRequest, head string) (forge.CI, error) {
		askedHead = head

		return forge.CI{State: forge.CIFailed}, nil
	}

	// Act
	announcement, _, err := compose(seams)

	// Assert
	if err != nil || announcement.Moment != messaging.MomentCIRed || askedHead != headCommit {
		t.Errorf("moment = %v (%v), head %q; want CI red, read at the branch's head", announcement.Moment, err, askedHead)
	}
}

func TestComposeAnnouncementTakesAnUnreadableCIAsReady(t *testing.T) {
	t.Parallel()

	cases := map[string]func(forge.PullRequest, string) (forge.CI, error){
		"no way to read CI": nil,
		"a CI read that fails": func(forge.PullRequest, string) (forge.CI, error) {
			return forge.CI{State: forge.CIFailed}, errSeam
		},
	}

	for name, check := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			seams := announceSeams()
			seams.CheckCI = check

			// Act
			announcement, _, err := compose(seams)

			// Assert
			if err != nil || announcement.Moment != messaging.MomentReady {
				t.Errorf("moment = %v (%v), want ready", announcement.Moment, err)
			}
		})
	}
}

func TestComposeAnnouncementReadsWithoutWhatItCannotSay(t *testing.T) {
	t.Parallel()

	// The author, the issue's summary and its link are extras: without them the
	// announcement still composes, saying less.
	cases := map[string]func(loop.AnnounceSeams) loop.AnnounceSeams{
		"no way to ask the forge or the tracker": func(seams loop.AnnounceSeams) loop.AnnounceSeams {
			seams.Author, seams.Issue, seams.BrowseURL = nil, nil, nil

			return seams
		},
		"a forge and a tracker that fail": func(seams loop.AnnounceSeams) loop.AnnounceSeams {
			seams.Author = func() (string, error) { return "", errSeam }
			seams.Issue = func(jira.Key) (jira.IssueDetail, error) { return jira.IssueDetail{}, errSeam }
			seams.BrowseURL = nil

			return seams
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			announcement, _, err := compose(mutate(announceSeams()))

			// Assert
			if err != nil || announcement.Author != "" || announcement.IssueSummary != "" ||
				announcement.IssueURL != "" || announcement.IssueKey != issueKey {
				t.Errorf("announcement = %+v (%v), want the key alone, with no author, summary or link", announcement, err)
			}
		})
	}
}

func TestComposeAnnouncementNamesNoIssueForAKeylessBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := announceSeams()
	seams.Branch = func() (gitrepo.Branch, error) { return openable(keylessBranch), nil }

	// Act
	announcement, _, err := compose(seams)

	// Assert
	if err != nil || announcement.IssueKey != "" || announcement.IssueSummary != "" || announcement.IssueURL != "" {
		t.Errorf("announcement = %+v (%v), want no issue named", announcement, err)
	}
}
