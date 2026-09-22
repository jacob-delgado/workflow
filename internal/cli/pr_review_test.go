// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// statusInReview is the review status these tests configure `pr` to offer.
const statusInReview = "In Review"

// reviewMoves offers In Progress then In Review, both indeterminate, so a test
// proves the offer is chosen by the destination name (In Review) and not by the
// first transition listed.
const reviewMoves = `{"transitions":[` +
	`{"id":"11","name":"Start Progress",` +
	`"to":{"name":"In Progress","statusCategory":{"key":"indeterminate"}},"fields":{}},` +
	`{"id":"31","name":"Start Review",` +
	`"to":{"name":"` + statusInReview + `","statusCategory":{"key":"indeterminate"}},"fields":{}}]}`

// applyRecord counts the transitions a fake Jira was asked to apply and keeps the
// last body it was sent. Both are atomic because `task test` runs -race and the
// handler answers on its own goroutine.
type applyRecord struct {
	count atomic.Int32
	body  atomic.Value
}

func (a *applyRecord) store(body string) {
	a.count.Add(1)
	a.body.Store(body)
}

func (a *applyRecord) applied() int32 {
	return a.count.Load()
}

func (a *applyRecord) sentBody() string {
	body, _ := a.body.Load().(string)

	return body
}

// reviewJira serves the requests a `pr` review-status offer makes: the issue read
// during compose, the transitions list, and applying one. It records any applied
// transition, so a test can prove which move was sent — or that none was.
func reviewJira(t *testing.T, transitions string) (string, *applyRecord) {
	t.Helper()

	record := &applyRecord{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.HasSuffix(request.URL.Path, "/transitions") && request.Method == http.MethodPost:
			body, _ := io.ReadAll(request.Body)
			record.store(string(body))
			writer.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(request.URL.Path, "/transitions"):
			_, _ = writer.Write([]byte(transitions))
		default:
			_, _ = writer.Write([]byte(issueFixture("PROJ-2", "Bug", "thing")))
		}
	}))
	t.Cleanup(server.Close)

	return server.URL, record
}

// reviewRepo is a published GitHub branch with no pull request yet, configured to
// offer statusInReview once one is opened, and its fake Jira's apply record.
func reviewRepo(t *testing.T, transitions string) (string, *applyRecord) {
	t.Helper()

	fakeGh(t, ghResponses{})
	baseURL, applied := reviewJira(t, transitions)
	repo := githubRepo(t, "fix/PROJ-2-thing")
	pretendPushed(t, repo)
	writeFile(t, repo, `{"jira":{"base_url":"`+baseURL+`","token":"t","review_status":"`+statusInReview+`"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	return repo, applied
}

func TestPRMovesTheIssueToTheReviewStatus(t *testing.T) {
	// Arrange
	repo, applied := reviewRepo(t, reviewMoves)

	// Act
	output, err := run(t, repo, "pr", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("pr --yes: %v (%s)", err, output)
	}

	if !strings.Contains(output, "Opened #7") || !strings.Contains(output, "Moved PROJ-2 to "+statusInReview) {
		t.Errorf("output does not open the pull request and move the issue to review:\n%s", output)
	}

	// The offer is chosen by the destination status name, so the In Review
	// transition (31) is applied — not the first-listed In Progress (11).
	if applied.applied() != 1 || !strings.Contains(applied.sentBody(), `"id":"31"`) {
		t.Errorf("applied = %d, body = %q; want the In Review transition sent once",
			applied.applied(), applied.sentBody())
	}
}

func TestPRLeavesTheIssueWhenTheReviewMoveIsDeclined(t *testing.T) {
	// Arrange
	repo, applied := reviewRepo(t, reviewMoves)

	// Act
	// Yes to opening the pull request, no to moving the issue.
	output, err := runGuided(t, repo, scripted([]string{"y", "n"}, nil), "pr")
	// Assert
	if err != nil {
		t.Fatalf("pr: %v (%s)", err, output)
	}

	if !strings.Contains(output, "Opened #7") || !strings.Contains(output, "Left PROJ-2 as it is.") {
		t.Errorf("output does not open the pull request and leave the issue:\n%s", output)
	}

	if applied.applied() != 0 {
		t.Errorf("a declined move still applied a transition (%d)", applied.applied())
	}
}

func TestPRDoesNotMoveToAReviewStatusItCannotComplete(t *testing.T) {
	// A workflow that never reaches In Review, and one that gates it behind a
	// required field the command cannot fill: both leave the issue untouched.
	noReview := `{"transitions":[{"id":"11","name":"Start Progress",` +
		`"to":{"name":"In Progress","statusCategory":{"key":"indeterminate"}},"fields":{}}]}`
	needsField := `{"transitions":[{"id":"31","name":"Start Review",` +
		`"to":{"name":"` + statusInReview + `","statusCategory":{"key":"indeterminate"}},` +
		`"fields":{"resolution":{"required":true,"hasDefaultValue":false,"name":"Resolution",` +
		`"schema":{"type":"string"}}}}]}`

	cases := map[string]string{
		"the status is not offered":    noReview,
		"the transition needs a field": needsField,
	}

	for name, transitions := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			repo, applied := reviewRepo(t, transitions)

			// Act
			output, err := run(t, repo, "pr", "--yes")
			// Assert
			if err != nil {
				t.Fatalf("pr --yes: %v (%s)", err, output)
			}

			if !strings.Contains(output, "Opened #7") {
				t.Errorf("the pull request should still open:\n%s", output)
			}

			if strings.Contains(output, "Moved PROJ-2") || applied.applied() != 0 {
				t.Errorf("moved the issue to a review status it could not complete:\n%s", output)
			}
		})
	}
}
