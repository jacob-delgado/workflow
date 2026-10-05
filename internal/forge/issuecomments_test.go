// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
)

const (
	githubCommentsPath = githubIssuePath + "/comments"
	gitlabNotesPath    = gitlabIssuePath + "/notes"
	commentWritten     = "2026-10-01T10:00:00Z"
)

// written is the time every fake comment was written.
func written(t *testing.T) time.Time {
	t.Helper()

	at, err := time.Parse(time.RFC3339, commentWritten)
	if err != nil {
		t.Fatalf("parsing %s: %v", commentWritten, err)
	}

	return at
}

func TestReadIssueCountsItsComments(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo   forge.Repo
		number int
		routes map[string]string
	}{
		"a GitHub issue": {
			repo: githubRepo(), number: 42, routes: map[string]string{githubIssuePath: `{"number":42,"comments":3}`},
		},
		"a GitLab issue": {
			repo: gitlabRepo(), number: 7, routes: map[string]string{gitlabIssuePath: `{"iid":7,"user_notes_count":3}`},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := forgeRouting(t, tt.routes)

			// Act
			detail, err := client.ReadIssue(t.Context(), tt.repo, tt.number)

			// Assert
			if err != nil || detail.CommentCount != 3 {
				t.Errorf("ReadIssue = %+v, %v, want a count of 3 comments", detail, err)
			}
		})
	}
}

func TestRecentIssueCommentsReadsAGitHubThread(t *testing.T) {
	t.Parallel()

	// Arrange
	// A deleted account comes back as no user at all; GitHub shows it as ghost.
	client, seen := forgeRouting(t, map[string]string{githubCommentsPath: `[` +
		`{"user":{"login":"ana"},"body":"first","created_at":"` + commentWritten + `"},` +
		`{"user":null,"body":"from a deleted account","created_at":"` + commentWritten + `"}]`})

	// Act
	comments, err := client.RecentIssueComments(t.Context(), githubRepo(), 42, 2)

	// Assert
	want := []forge.IssueComment{
		{Author: userAna, Body: "first", Created: written(t)},
		{Author: userGhost, Body: "from a deleted account", Created: written(t)},
	}
	if err != nil || len(comments) != 2 || comments[0] != want[0] || comments[1] != want[1] {
		t.Errorf("IssueComments = %+v, %v, want %+v", comments, err, want)
	}

	if asked := requestTo(*seen, githubCommentsPath); asked.method != http.MethodGet {
		t.Errorf("asked %+v, want a GET of the issue's comments", asked)
	}
}

func TestRecentIssueCommentsReadsAGitLabThreadNewestFirstAndTurnsItAround(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := forgeRouting(t, map[string]string{gitlabNotesPath: `[` +
		`{"author":{"username":"ben"},"body":"the latest","created_at":"` + commentWritten + `","system":false},` +
		`{"author":{"username":"ben"},"body":"changed the milestone","created_at":"` + commentWritten +
		`","system":true},` +
		`{"author":{"username":"ben"},"body":"the first","created_at":"` + commentWritten + `","system":false}]`})

	// Act
	comments, err := client.RecentIssueComments(t.Context(), gitlabRepo(), 7, 2)

	// Assert
	if err != nil || len(comments) != 2 || comments[0].Body != "the first" || comments[1].Body != "the latest" {
		t.Errorf("RecentIssueComments = %+v, %v, want the two comments oldest first, no system note", comments, err)
	}

	asked := requestTo(*seen, gitlabNotesPath)
	if !strings.Contains(asked.query, "sort=desc") || !strings.Contains(asked.query, "order_by=created_at") {
		t.Errorf("asked %+v, want the notes newest first", asked)
	}
}

func TestCommentOnIssuePostsTheBodyAsWritten(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo   forge.Repo
		number int
		path   string
		answer string
	}{
		"a GitHub issue": {
			repo: githubRepo(), number: 42, path: githubCommentsPath,
			answer: `{"user":{"login":"ana"},"body":"**done**","created_at":"` + commentWritten + `"}`,
		},
		"a GitLab issue": {
			repo: gitlabRepo(), number: 7, path: gitlabNotesPath,
			answer: `{"author":{"username":"ana"},"body":"**done**","created_at":"` + commentWritten + `"}`,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, seen := forgeAnswering(t, http.StatusCreated, tt.answer)

			// Act
			posted, err := client.CommentOnIssue(t.Context(), tt.repo, tt.number, "**done**")

			// Assert
			sent := lastRequest(t, seen)
			if err != nil || sent.method != http.MethodPost || sent.path != tt.path || sent.body["body"] != "**done**" {
				t.Errorf("sent %+v (%v), want a POST of the body to %s", sent, err, tt.path)
			}

			want := forge.IssueComment{Author: userAna, Body: "**done**", Created: written(t)}
			if posted != want {
				t.Errorf("CommentOnIssue = %+v, want %+v", posted, want)
			}
		})
	}
}

func TestAGitLabNoteOfOnlyQuickActionsIsAcceptedNotFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitLab applies the actions and answers 202 with no note; reading that as
	// a failure would invite a retry that applies them twice.
	client, _ := forgeAnswering(t, http.StatusAccepted, `{"commands_changes":{"state_event":"close"}}`)

	// Act
	posted, err := client.CommentOnIssue(t.Context(), gitlabRepo(), 7, "/close")

	// Assert
	if err != nil || posted != (forge.IssueComment{}) {
		t.Errorf("CommentOnIssue = %+v, %v, want no comment and no error", posted, err)
	}
}

func TestACommentTheForgeTurnsDownSaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := forgeAnswering(t, http.StatusUnprocessableEntity, `{"message":"Body is too long"}`)

	// Act
	_, err := client.CommentOnIssue(t.Context(), githubRepo(), 42, "far too long")

	// Assert
	if !errors.Is(err, forge.ErrRejected) || !strings.Contains(err.Error(), "Body is too long") {
		t.Errorf("CommentOnIssue error = %v, want a rejection carrying the forge's reason", err)
	}
}

func TestAServerErrorIsNotPassedOnAsTheForgesReason(t *testing.T) {
	t.Parallel()

	// Arrange
	// A gateway in front of the forge answers 5xx in JSON of its own, which
	// can name a host behind it; only a 4xx is the forge explaining a refusal.
	client, _ := forgeAnswering(t, http.StatusBadGateway, `{"message":"upstream db-7.internal is down"}`)

	// Act
	_, err := client.CommentOnIssue(t.Context(), githubRepo(), 42, "hi")

	// Assert
	if errors.Is(err, forge.ErrRejected) || !errors.Is(err, forge.ErrUnexpectedStatus) ||
		strings.Contains(err.Error(), "db-7.internal") {
		t.Errorf("CommentOnIssue error = %v, want an undocumented status without the gateway's words", err)
	}
}

func TestA202IsSuccessOnlyForAGitLabComment(t *testing.T) {
	t.Parallel()

	// Arrange
	// A 202 elsewhere means work still running, which is not yet done.
	client, _ := forgeAnswering(t, http.StatusAccepted, `{}`)

	// Act
	err := client.CloseIssue(t.Context(), gitlabRepo(), 7)

	// Assert
	if err == nil {
		t.Error("CloseIssue answered 202 = nil, want it not read as done")
	}
}

func TestRecentIssueCommentsReadsAGitHubThreadFromItsLastPages(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitHub lists a thread oldest first with no other order, so the newest
	// hundred of 150 comments are the last page's 50 and the 50 before them.
	comment := func(number int) string {
		return `{"user":{"login":"ana"},"body":"comment ` + strconv.Itoa(number) + `","created_at":"` +
			commentWritten + `"}`
	}
	client := forgePaging(t, map[string][]string{githubCommentsPath: {
		listingOf(1, 100, comment), listingOf(101, 50, comment),
	}})

	// Act
	comments, err := client.RecentIssueComments(t.Context(), githubRepo(), 42, 150)

	// Assert
	if err != nil || len(comments) != 100 || comments[0].Body != "comment 51" || comments[99].Body != "comment 150" {
		t.Errorf("RecentIssueComments = %d comments, %v; want 51 to 150, oldest first", len(comments), err)
	}
}
