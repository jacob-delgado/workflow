// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// trackerPage is the tracker's page for an issue, as Deps.BrowseURL gives it.
func trackerPage(key jira.Key) string {
	return "https://jira.example.com/browse/" + string(key)
}

func TestATasksIssueLinkIsTheTrackersPage(t *testing.T) {
	t.Parallel()

	// A task's jiraurl is Taskwarrior's data, which any replica or hand edit
	// can have written: the terminal opens the tracker's page, and so does the
	// page.
	cases := map[string]struct {
		issueKey, jiraURL string
		tracker           bool
		want              string
	}{
		"the tracker's page over a stale jiraurl": {
			issueKey: testKey, jiraURL: "https://old.example.com/388", tracker: true, want: trackerPage(testKey),
		},
		"the tracker's page for a task linked by jiraid alone": {
			issueKey: testKey, tracker: true, want: trackerPage(testKey),
		},
		"the jiraurl with no tracker link": {
			issueKey: testKey, jiraURL: "https://jira.example/browse/" + testKey,
			want: "https://jira.example/browse/" + testKey,
		},
		"the jiraurl of a task naming no issue": {
			jiraURL: "http://jira.example/browse/PROJ-9", tracker: true, want: "http://jira.example/browse/PROJ-9",
		},
		"no link for a jiraurl that is not a web address": {issueKey: testKey, jiraURL: "javascript:alert(1)"},
		"no link for a jiraurl with no host":              {issueKey: testKey, jiraURL: "https:///browse/PROJ-412"},
		"no link for a relative jiraurl":                  {issueKey: testKey, jiraURL: "/browse/PROJ-412"},
		"no link for a task naming neither":               {tracker: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			task := plainTask()
			task.IssueKey, task.IssueURL = tt.issueKey, tt.jiraURL

			fake := fakeTaskwarrior()
			fake.pending.Tasks = []taskwarrior.Task{task}

			deps := tasksDeps(fake)
			if tt.tracker {
				deps.Jira.BrowseURL = trackerPage
			}

			// Act
			recorder := get(t, serve(t, deps, config.Default()), tasksPath)

			// Assert
			list := decode[api.TaskList](t, recorder)
			if recorder.Code != http.StatusOK || len(list.Tasks) != 1 {
				t.Fatalf("answer = %d %+v, want 200 with the one task", recorder.Code, list)
			}

			if got := list.Tasks[0].IssueURL; got != tt.want {
				t.Errorf("issue_url = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTheSnapshotsTaskLinksAreTheTrackersPage(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := fakeTaskwarrior()
	for index := range fake.linked {
		fake.linked[index].IssueURL = "https://old.example.com/" + fake.linked[index].IssueKey
	}

	deps := tasksDeps(fake)
	deps.Jira.BrowseURL = trackerPage

	// Act
	body := streamOnce(t, serve(t, deps, config.Default()), "/api/events").Body.String()

	// Assert
	summary := firstSnapshot(t, body).Tasks
	if summary.Active == nil || summary.Active.IssueURL != trackerPage(testKey) {
		t.Errorf("active = %+v, want its issue_url the tracker's page %q", summary.Active, trackerPage(testKey))
	}

	for _, task := range summary.Linked {
		if want := trackerPage(jira.Key(task.IssueKey)); task.IssueURL != want {
			t.Errorf("linked task %s has issue_url %q, want the tracker's page %q", task.UUID, task.IssueURL, want)
		}
	}
}
