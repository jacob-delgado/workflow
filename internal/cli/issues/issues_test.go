// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package issues_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/issues"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// scopedSprint is the sprint view's JQL as it is searched: scoped to you.
const scopedSprint = "(sprint in openSprints()) AND assignee = currentUser()"

// searchOf is the Jira body for a search finding keys of total.
func searchOf(total int, keys ...string) string {
	page := clitest.SearchFixture(keys...)

	return `{"total":` + strconv.Itoa(total) + strings.TrimPrefix(page, `{"total":`+strconv.Itoa(len(keys)))
}

// decoded is stdout decoded as one T, failing t when it is not.
func decoded[T any](t *testing.T, stdout string) T {
	t.Helper()

	var value T

	err := json.Unmarshal([]byte(stdout), &value)
	if err != nil {
		t.Fatalf("stdout is not a %T: %v\n%s", value, err, stdout)
	}

	return value
}

func TestEachIssueCommandIsNamedForWhatItDoes(t *testing.T) {
	t.Parallel()

	prompt := clitest.UnusedPrompt(t)
	cases := map[string]*cobra.Command{
		"issues":     issues.NewIssuesCmd(),
		"issue":      issues.NewIssueCmd(),
		"transition": issues.NewTransitionCmd(prompt),
		"assign":     issues.NewAssignCmd(prompt),
		"worklog":    issues.NewWorklogCmd(prompt),
		"comment":    issues.NewCommentCmd(prompt),
	}

	for want, cmd := range cases {
		t.Run(want, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := cmd.Name(); got != want {
				t.Errorf("command named %q, want %q", got, want)
			}
		})
	}
}

func TestIssuesListsTheFirstViewScopedToYou(t *testing.T) {
	t.Parallel()

	// Arrange
	baseURL, asked := trackerJira(t, trackerAnswers{search: clitest.SearchFixture("PROJ-7", "PROJ-8")})
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, sprintAndTriage))

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "issues")
	// Assert
	if err != nil {
		t.Fatalf("issues: %v (%+v)", err, printed)
	}

	if got := asked.queries(); !slices.Equal(got, []string{scopedSprint}) {
		t.Errorf("searched %q, want the first view scoped to you, once", got)
	}

	want := "PROJ-7  x  (To Do)\nPROJ-8  x  (To Do)\n"
	if printed.Stdout != want {
		t.Errorf("stdout = %q, want one line per issue:\n%s", printed.Stdout, want)
	}
}

func TestIssuesWithNoViewsListsTheIssuesAssignedToYou(t *testing.T) {
	t.Parallel()

	// Arrange
	baseURL, asked := trackerJira(t, trackerAnswers{search: clitest.SearchFixture("PROJ-7")})
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, ""))

	// Act
	_, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "issues")

	// Assert
	if got := asked.queries(); err != nil || !slices.Equal(got, []string{jira.AssignedToMe}) {
		t.Errorf("issues = %v, searched %q; want the issues assigned to you", err, got)
	}
}

func TestIssuesViewFlagListsTheNamedViewAsItIs(t *testing.T) {
	t.Parallel()

	// Arrange
	baseURL, asked := trackerJira(t, trackerAnswers{search: clitest.SearchFixture("PROJ-7")})
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, sprintAndTriage))

	// Act
	_, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "issues", "--view", "Triage")

	// Assert
	// The triage view names the assignee, so it is searched as configured.
	if got := asked.queries(); err != nil || !slices.Equal(got, []string{"assignee is EMPTY"}) {
		t.Errorf("issues --view Triage = %v, searched %q; want the view's own JQL", err, got)
	}
}

func TestIssuesRefusesAViewNoConfigurationNames(t *testing.T) {
	t.Parallel()

	// Arrange
	baseURL, asked := trackerJira(t, trackerAnswers{search: clitest.SearchFixture("PROJ-7")})
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, sprintAndTriage))

	// Act
	_, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "issues", "--view", "sprint")

	// Assert
	wantExit(t, err, 2)

	if err == nil || !strings.Contains(err.Error(), "Sprint, Triage") || len(asked.queries()) != 0 {
		t.Errorf("issues --view sprint = %v, searched %q; want the views named and Jira not asked", err, asked.queries())
	}
}

func TestIssuesJSONPrintsTheWebsIssuesPage(t *testing.T) {
	t.Parallel()

	// Arrange
	baseURL, _ := trackerJira(t, trackerAnswers{search: clitest.SearchFixture("PROJ-7")})
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, ""))

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "issues", "--json")
	// Assert
	if err != nil {
		t.Fatalf("issues --json: %v (%+v)", err, printed)
	}

	page := decoded[api.IssuesPage](t, printed.Stdout)

	if len(page.Issues) != 1 || page.Issues[0].Key != "PROJ-7" || page.Issues[0].Tracker != api.IssueTrackerJira ||
		page.Total != 1 || !strings.Contains(printed.Stdout, `"unavailable": []`) {
		t.Errorf("issues --json = %s, want PROJ-7 from Jira, total 1, nothing unavailable", printed.Stdout)
	}
}

func TestIssuesListsTheForgesIssuesBesideTheFirstView(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Search: `{"total_count":1,"items":[{"number":42,` +
		`"html_url":"https://github.com/owner/repo/issues/42","title":"forge bug"}]}`})

	baseURL, _ := trackerJira(t, trackerAnswers{search: clitest.SearchFixture("PROJ-7")})
	repo := clitest.GithubRepo(t, "work")
	clitest.WriteFile(t, repo, `{"jira":{"base_url":"`+baseURL+`","token":"t"`+sprintAndTriage+`},`+
		`"issues":{"forge":true},"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "issues", "--json")
	// Assert
	// The command's JQL is the first view's, so the forge's issues join it as
	// they join the interfaces' first list.
	if err != nil {
		t.Fatalf("issues --json: %v (%+v)", err, printed)
	}

	page := decoded[api.IssuesPage](t, printed.Stdout)

	forgeListed := slices.ContainsFunc(page.Issues, func(issue api.Issue) bool {
		return issue.Key == "42" && issue.Tracker == api.IssueTrackerForge
	})
	if !forgeListed {
		t.Errorf("issues --json = %s, want forge issue 42 beside Jira's", printed.Stdout)
	}
}

func TestIssuesSaysWhichTrackerCouldNotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Search: "{"})

	baseURL, _ := trackerJira(t, trackerAnswers{search: clitest.SearchFixture("PROJ-7")})
	repo := clitest.GithubRepo(t, "work")
	clitest.WriteFile(t, repo, `{"jira":{"base_url":"`+baseURL+`","token":"t"},`+
		`"issues":{"forge":true},"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "issues")

	// Assert
	// Jira's issues are listed all the same, as the web answers 200.
	if err != nil || !strings.Contains(printed.Stdout, "PROJ-7") ||
		!strings.Contains(printed.Stderr, "Not read: the forge's issues.") {
		t.Errorf("issues = %v\nstdout:\n%s\nstderr:\n%s\nwant Jira's issues and the forge named unread",
			err, printed.Stdout, printed.Stderr)
	}
}

func TestIssuesSaysWhenTheViewIsEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	baseURL, _ := trackerJira(t, trackerAnswers{search: clitest.SearchFixture()})
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, sprintAndTriage))

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "issues")

	// Assert
	if err != nil || printed.Stdout != "" || !strings.Contains(printed.Stderr, "No issues in Sprint.") {
		t.Errorf("issues = %v\nstdout:\n%s\nstderr:\n%s\nwant nothing listed and the view named empty",
			err, printed.Stdout, printed.Stderr)
	}
}

func TestIssuesNotesAListCutShort(t *testing.T) {
	t.Parallel()

	// Arrange
	baseURL, _ := trackerJira(t, trackerAnswers{search: searchOf(120, "PROJ-7", "PROJ-8")})
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, sprintAndTriage))

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "issues")

	// Assert
	if err != nil || !strings.Contains(printed.Stderr, "Listed the first 2 of 120 issues in Sprint.") {
		t.Errorf("issues = %v\nstderr:\n%s\nwant the list noted as cut short", err, printed.Stderr)
	}
}

func TestIssuesSaysWhyTheViewCouldNotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	baseURL, _ := trackerJira(t, trackerAnswers{search: `{}`, searchCode: http.StatusInternalServerError})
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, sprintAndTriage))

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "issues")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "listing Sprint") || printed.Stdout != "" {
		t.Errorf("issues = %v, stdout %q; want the failed read named and nothing listed", err, printed.Stdout)
	}
}

func TestIssueJSONPrintsTheWebsIssueDetail(t *testing.T) {
	t.Parallel()

	// Arrange
	baseURL, _ := trackerJira(t, trackerAnswers{})
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, ""))

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "issue", trackedIssue, "--json")
	// Assert
	if err != nil {
		t.Fatalf("issue --json: %v (%+v)", err, printed)
	}

	detail := decoded[api.IssueDetail](t, printed.Stdout)

	if detail.Key != trackedIssue || detail.URL != baseURL+"/browse/"+trackedIssue || len(detail.Comments) != 2 ||
		detail.CommentTotal != 3 || detail.Assignee == nil || *detail.Assignee != "Bo Chen" {
		t.Errorf("issue --json = %s, want PROJ-7 in full, with its link and comments", printed.Stdout)
	}
}

func TestIssueJSONLinksTheKeyTheTrackerAnsweredWith(t *testing.T) {
	t.Parallel()

	// Arrange
	baseURL, _ := trackerJira(t, trackerAnswers{})
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, ""))

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "issue", movedIssue, "--json")
	// Assert
	if err != nil {
		t.Fatalf("issue --json: %v (%+v)", err, printed)
	}

	// GET /api/issues/{key} links the key Jira answered with, not the one asked.
	detail := decoded[api.IssueDetail](t, printed.Stdout)
	if detail.Key != trackedIssue || detail.URL != baseURL+"/browse/"+trackedIssue {
		t.Errorf("issue %s --json = key %q, url %q; want %s and its link, as the web answers",
			movedIssue, detail.Key, detail.URL, trackedIssue)
	}
}

func TestIssuePrintsTheIssueInFull(t *testing.T) {
	t.Parallel()

	// Arrange
	baseURL, _ := trackerJira(t, trackerAnswers{})
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, ""))

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "issue", trackedIssue)
	// Assert
	if err != nil {
		t.Fatalf("issue: %v (%+v)", err, printed)
	}

	for _, want := range []string{
		"PROJ-7  Fix login", "Bug · In Progress · High", "Reported by Ana Lopez, assigned to Bo Chen",
		baseURL + "/browse/PROJ-7", "Login fails when the token is empty.",
		"Ana Lopez · 2026-09-03", "  Repro'd on 8.2.1", "2 of 3 comments shown.",
	} {
		if !strings.Contains(printed.Stdout, want) {
			t.Errorf("issue printed:\n%s\nwant it to hold %q", printed.Stdout, want)
		}
	}
}

func TestIssuePrintsOnlyWhatItHasOfASparseIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	baseURL, _ := trackerJira(t, trackerAnswers{})
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, ""))

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "issue", sparseIssue)

	// Assert
	want := "PROJ-8  Bare\nTo Do\nReported by nobody named, unassigned\n" + baseURL + "/browse/PROJ-8\n" +
		"\nsomeone\n  undated\n"
	if err != nil || printed.Stdout != want {
		t.Errorf("issue = %v, printed:\n%s\nwant:\n%s", err, printed.Stdout, want)
	}
}

func TestIssuePrintsNoLinkForAnIssueWithNoPage(t *testing.T) {
	t.Parallel()

	// Arrange
	baseURL, _ := trackerJira(t, trackerAnswers{})
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, ""))

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "issue", unlinkedIssue)

	// Assert
	want := "11  Unlinked\nTo Do\nReported by nobody named, unassigned\n"
	if err != nil || printed.Stdout != want {
		t.Errorf("issue = %v, printed:\n%s\nwant:\n%s", err, printed.Stdout, want)
	}
}

func TestIssueNotInTheTrackerExitsAsAFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	baseURL, _ := trackerJira(t, trackerAnswers{})
	dir := t.TempDir()
	clitest.WriteFile(t, dir, trackerConfig(baseURL, ""))

	// Act
	_, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "issue", "PROJ-9")

	// Assert
	wantExit(t, err, 1)

	if err == nil || !strings.Contains(err.Error(), "reading issue PROJ-9") {
		t.Errorf("issue PROJ-9 = %v, want the failed read named", err)
	}
}

func TestIssueCommandsCompleteTheirKey(t *testing.T) {
	t.Parallel()

	for command := range strings.FieldsSeq("branch comment issue transition assign worklog") {
		t.Run(command, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server := clitest.JiraServer(t, http.StatusOK, clitest.SearchFixture("PROJ-7", "PROJ-8"), new(atomic.Bool))
			dir := t.TempDir()
			clitest.WriteConfigFor(t, dir, server.URL)

			// Act
			offered, err := clitest.Run(t, dir, "__complete", command, "")

			// Assert
			if err != nil || !strings.Contains(offered, "PROJ-7") || !strings.Contains(offered, "PROJ-8") {
				t.Errorf("__complete %s = %v, offered:\n%s\nwant the assigned issues", command, err, offered)
			}
		})
	}
}

func TestIssueCommandsCompleteNothingPastTheKey(t *testing.T) {
	t.Parallel()

	// Arrange
	reached := new(atomic.Bool)
	server := clitest.JiraServer(t, http.StatusOK, clitest.SearchFixture("PROJ-7", "PROJ-8"), reached)
	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, server.URL)

	// Act
	offered, err := clitest.Run(t, dir, "__complete", "transition", "PROJ-7", "")

	// Assert
	if err != nil || strings.Contains(offered, "PROJ-8") || reached.Load() {
		t.Errorf("__complete transition PROJ-7 = %v, offered:\n%s\nwant no key offered for the status, Jira not asked",
			err, offered)
	}
}
