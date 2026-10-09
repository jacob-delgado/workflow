// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// With Jira configured and a repository opting into its forge's issues, the
// Issues list draws on both: these drive that combined tracker through
// wiring.Deps against a stand-in Jira and a stand-in gh.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// jiraKey is the one Jira issue the stand-in lists.
const jiraKey jira.Key = "PROJ-1"

// jiraPage is the search answer the stand-in Jira gives: one of its three
// assigned issues.
const jiraPage = `{"total":3,"issues":[{"key":"PROJ-1","fields":{"summary":"the Jira one",` +
	`"status":{"name":"To Do","statusCategory":{"key":"new"}}}}]}`

// fakeJira is a Jira answering every search with jiraPage and every issue
// read with PROJ-1, keeping each request it is asked.
type fakeJira struct {
	lock  sync.Mutex
	asked []*http.Request
}

// serve starts the stand-in and returns its address.
func (j *fakeJira) serve(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		j.lock.Lock()
		j.asked = append(j.asked, request)
		j.lock.Unlock()

		writer.Header().Set("Content-Type", "application/json")

		if strings.Contains(request.URL.Path, "/search") {
			_, _ = writer.Write([]byte(jiraPage))

			return
		}

		_, _ = writer.Write([]byte(`{"key":"PROJ-1","fields":{"summary":"the Jira one"}}`))
	}))
	t.Cleanup(server.Close)

	return server.URL
}

// requests is every request the stand-in was asked, as "METHOD path?query".
func (j *fakeJira) requests() []string {
	j.lock.Lock()
	defer j.lock.Unlock()

	described := make([]string, 0, len(j.asked))
	for _, request := range j.asked {
		described = append(described, request.Method+" "+request.URL.Path+"?"+request.URL.RawQuery)
	}

	return described
}

// pullURL is the pull request the tests link to an issue.
const pullURL = "https://github.com/owner/repo/pull/7"

// bothTrackers is the tracker over the stand-in Jira and the forge's issues
// for owner/repo on github.com, through gh.
func bothTrackers(t *testing.T, stand *fakeJira) seams.Jira {
	t.Helper()

	return trackersOver(t, stand.serve(t))
}

// trackersOver is the tracker over the Jira at baseURL and the forge's issues
// for owner/repo on github.com, through gh.
func trackersOver(t *testing.T, baseURL string) seams.Jira {
	t.Helper()

	cfg := config.Default()
	cfg.Jira = config.Jira{BaseURL: baseURL, Token: jiraToken}
	cfg.Forge = config.Forge{CLI: true, Kind: githubKind, Host: hostGitHub}
	cfg.Issues.Forge = true

	return wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: remoteGitHub}, nil).Jira
}

// unavailableJira is a Jira that answers every request with trouble of its
// own, and its address.
func unavailableJira(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	return server.URL
}

// keysOf are the keys of the rows a search answered, in order.
func keysOf(result jira.SearchResult) []jira.Key {
	keys := make([]jira.Key, 0, len(result.Issues))
	for _, issue := range result.Issues {
		keys = append(keys, issue.Key)
	}

	return keys
}

func TestBothTrackersListTheFirstViewTogether(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{search: theBugList})
	tracker := bothTrackers(t, &fakeJira{})

	// Act
	result, err := tracker.Search(jira.AssignedToMe, 0)

	// Assert
	if err != nil || !slices.Equal(keysOf(result), []jira.Key{"42", jiraKey}) || result.Total != 4 {
		t.Errorf("Search = %v of %d, %v; want the forge's 42 then Jira's PROJ-1, of 3 + 1", keysOf(result), result.Total, err)
	}
}

func TestALaterPageAsksJiraPastItsOwnRows(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{search: theBugList})

	stand := &fakeJira{}
	tracker := bothTrackers(t, stand)

	_, err := tracker.Search(jira.AssignedToMe, 0)
	if err != nil {
		t.Fatalf("the first page: %v", err)
	}

	// Act
	// Two rows are listed, one of them the forge's, so Jira has given one.
	_, err = tracker.Search(jira.AssignedToMe, 2)

	// Assert
	asked := stand.requests()
	if err != nil || !strings.Contains(asked[len(asked)-1], "startAt=1") {
		t.Errorf("Search = %v; Jira was last asked %q, want from its second row", err, asked[len(asked)-1])
	}
}

func TestAnotherViewListsJiraAlone(t *testing.T) {
	// Arrange
	ghStub := installForgeCLI(t, "gh", forgeReplies{search: theBugList})
	tracker := bothTrackers(t, &fakeJira{})

	// Act
	result, err := tracker.Search("project = TEAM", 0)

	// Assert
	if err != nil || !slices.Equal(keysOf(result), []jira.Key{jiraKey}) || len(ghStub.args()) != 0 {
		t.Errorf("Search = %v, %v, gh asked %v; want Jira's rows alone", keysOf(result), err, ghStub.args())
	}
}

func TestAKeyCheckOfForgeNumbersAloneAsksOnlyTheForge(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{search: theBugList})

	stand := &fakeJira{}
	tracker := bothTrackers(t, stand)

	// Act
	result, err := tracker.SearchLenient(jira.NoKeys, 0)

	// Assert
	if err != nil || !slices.Equal(keysOf(result), []jira.Key{"42"}) || len(stand.requests()) != 0 {
		t.Errorf("SearchLenient = %v, %v, Jira asked %v; want the forge's 42 alone", keysOf(result), err, stand.requests())
	}
}

func TestJiraAloneIsNotAskedAKeyCheckNamingNoKey(t *testing.T) {
	// Arrange
	stand := &fakeJira{}

	cfg := config.Default()
	cfg.Jira = config.Jira{BaseURL: stand.serve(t), Token: jiraToken}
	tracker := wired(t, cfg, wiring.Workspace{Root: t.TempDir()}, nil).Jira

	// Act
	result, err := tracker.SearchLenient(jira.NoKeys, 0)

	// Assert
	if err != nil || len(result.Issues) != 0 || len(stand.requests()) != 0 {
		t.Errorf("SearchLenient = %v, %v, Jira asked %v; want nothing asked", keysOf(result), err, stand.requests())
	}
}

func TestEachKeyIsReadFromItsOwnTracker(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{issue: theBugDetail})

	stand := &fakeJira{}
	tracker := bothTrackers(t, stand)

	// Act
	forgeIssue, forgeErr := tracker.Issue("42")
	jiraIssue, jiraErr := tracker.Issue(jiraKey)

	// Assert
	if forgeErr != nil || forgeIssue.Description != "it broke" {
		t.Errorf("Issue(42) = %+v, %v; want the forge's issue", forgeIssue, forgeErr)
	}

	if jiraErr != nil || jiraIssue.Issue.Key != jiraKey || slices.ContainsFunc(stand.requests(), func(asked string) bool {
		return strings.Contains(asked, "/issue/42")
	}) {
		t.Errorf("Issue(PROJ-1) = %+v, %v, Jira asked %v; want Jira's issue, and 42 never sent to Jira",
			jiraIssue.Issue, jiraErr, stand.requests())
	}
}

func TestAForgeThatCannotBeReadLeavesJirasIssues(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{fail: true})
	tracker := bothTrackers(t, &fakeJira{})

	// Act
	result, err := tracker.Search(jira.AssignedToMe, 0)

	// Assert
	if err != nil || !slices.Equal(keysOf(result), []jira.Key{jiraKey}) || len(result.Unavailable) != 1 {
		t.Errorf("Search = %v, unavailable %v, %v; want Jira's rows and the forge named unavailable",
			keysOf(result), result.Unavailable, err)
	}
}

func TestACommentOnAForgeIssueGoesToTheForgeNotJira(t *testing.T) {
	// Arrange
	ghStub := installForgeCLI(t, "gh", forgeReplies{comments: `{"user":{"login":"octo"},"body":"on it"}`})

	stand := &fakeJira{}
	tracker := bothTrackers(t, stand)

	// Act
	posted, err := tracker.Comment("42", "on it")

	// Assert
	if err != nil || posted.Author != "octo" || posted.Body != "on it" || len(stand.requests()) != 0 {
		t.Errorf("Comment(42) = %+v, %v, Jira asked %v; want it posted on the forge alone",
			posted, err, stand.requests())
	}

	if args := ghStub.args(); !strings.Contains(args[len(args)-1], "/issues/42/comments") ||
		!strings.Contains(ghStub.stdin(), "on it") {
		t.Errorf("gh was called as %v with %q, want the comment posted to issue 42", args, ghStub.stdin())
	}
}

func TestTheForgeTrackerLinksAndAssignsAnIssue(t *testing.T) {
	// Arrange
	ghStub := installForgeCLI(t, "gh", forgeReplies{issue: theBugDetail})
	tracker := forgeTracker(t)

	// Act
	link := tracker.BrowseURL("42")
	err := tracker.Assign("42", "ana")

	// Assert
	if link != "https://github.com/owner/repo/issues/42" {
		t.Errorf("BrowseURL(42) = %q, want the issue's page", link)
	}

	args := ghStub.args()
	if err != nil || !containsAll(args, "-X", "POST") || !strings.Contains(args[len(args)-1], "/issues/42/assignees") {
		t.Errorf("Assign = %v; gh was called as %v, want a POST to 42's assignees", err, args)
	}
}

func TestJiraAloneAsksNothingOfAForgeNumber(t *testing.T) {
	t.Parallel()

	// Jira's REST API reads a bare number as an issue's id, so a forge issue's
	// 42 sent there would change whichever Jira issue has that id. Every seam
	// that names an issue refuses it as missing, before Jira is asked.
	cases := map[string]func(tracker seams.Jira) error{
		seamIssue: func(tracker seams.Jira) error {
			_, err := tracker.Issue("42")

			return err
		},
		seamTransitions: func(tracker seams.Jira) error {
			_, err := tracker.Transitions("42")

			return err
		},
		seamTransition: func(tracker seams.Jira) error {
			return tracker.Transition("42", jira.Transition{ID: "31"}, nil)
		},
		seamComment: func(tracker seams.Jira) error {
			_, err := tracker.Comment("42", "on it")

			return err
		},
		seamAssign: func(tracker seams.Jira) error { return tracker.Assign("42", "ana") },
		seamAddWorklog: func(tracker seams.Jira) error {
			_, err := tracker.AddWorklog("42", "1h", "")

			return err
		},
		seamLinkPull: func(tracker seams.Jira) error {
			return tracker.LinkPullRequest("42", pullURL, "work")
		},
	}

	for name, ask := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			stand := &fakeJira{}
			cfg := config.Default()
			cfg.Jira = config.Jira{BaseURL: stand.serve(t), Token: jiraToken}
			tracker := wired(t, cfg, wiring.Workspace{Root: t.TempDir()}, nil).Jira

			// Act
			err := ask(tracker)

			// Assert
			if !errors.Is(err, jira.ErrNotFound) || len(stand.requests()) != 0 {
				t.Errorf("%s(42) = %v, Jira asked %v; want jira.ErrNotFound before Jira is asked", name, err, stand.requests())
			}
		})
	}
}

func TestOnlyAJiraIssueTakesAWorklogOrAPullRequestLink(t *testing.T) {
	// A forge issue keeps its time and its links on its own page, so the
	// combined tracker refuses its number before either tracker is asked.
	cases := map[string]func(tracker seams.Jira) error{
		seamAddWorklog: func(tracker seams.Jira) error {
			_, err := tracker.AddWorklog("42", "1h", "")

			return err
		},
		seamLinkPull: func(tracker seams.Jira) error {
			return tracker.LinkPullRequest("42", pullURL, "work")
		},
	}

	for name, ask := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			ghStub := installForgeCLI(t, "gh", forgeReplies{})
			stand := &fakeJira{}
			tracker := bothTrackers(t, stand)

			// Act
			err := ask(tracker)

			// Assert
			if err == nil || !strings.Contains(err.Error(), "only a Jira issue takes this") ||
				len(stand.requests()) != 0 || len(ghStub.args()) != 0 {
				t.Errorf("%s(42) = %v, Jira asked %v, gh asked %v; want it refused before either is asked",
					name, err, stand.requests(), ghStub.args())
			}
		})
	}
}

func TestAJiraIssuesWorklogGoesToJira(t *testing.T) {
	// Arrange
	installForgeCLI(t, "gh", forgeReplies{})

	stand := &fakeJira{}
	tracker := bothTrackers(t, stand)

	// Act
	_, err := tracker.AddWorklog(jiraKey, "1h", "")

	// Assert
	if err != nil || !slices.ContainsFunc(stand.requests(), func(asked string) bool {
		return strings.HasPrefix(asked, "POST ") && strings.Contains(asked, "/issue/PROJ-1/worklog")
	}) {
		t.Errorf("AddWorklog(PROJ-1) = %v, Jira asked %v; want the worklog posted to Jira", err, stand.requests())
	}
}

func TestAJiraThatCannotBeReadFailsTheList(t *testing.T) {
	// Jira's rows are the list, the forge's only lead it, so a Jira that
	// cannot be read fails any page of it.
	for name, startAt := range map[string]int{"the first page": 0, "a later page": 2} {
		t.Run(name, func(t *testing.T) {
			// Arrange
			installForgeCLI(t, "gh", forgeReplies{search: theBugList})
			tracker := trackersOver(t, unavailableJira(t))

			// Act
			result, err := tracker.Search(jira.AssignedToMe, startAt)

			// Assert
			if err == nil || len(result.Issues) != 0 {
				t.Errorf("Search from %d = %v, %v; want Jira's failure and nothing listed", startAt, keysOf(result), err)
			}
		})
	}
}
