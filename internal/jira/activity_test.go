// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// leakKey and leakSummary are the issue activityIssue answers.
const (
	leakKey     = "PROJ-7"
	leakSummary = "Fix the token leak"
)

// activityIssue is PROJ-7 as a changelog-expanded search answers it: reported
// by me inside the period, moved by me and by someone else, worked on and
// commented on by me inside it and once before it.
const activityIssue = `{"key":"PROJ-7","fields":{"summary":"` + leakSummary + `",` +
	`"created":"2026-10-02T09:00:00.000+0000","reporter":{"name":"ana"},` +
	`"comment":{"total":2,"comments":[` +
	`{"author":{"name":"ana"},"body":"on it","created":"2026-10-02T10:00:00.000+0000"},` +
	`{"author":{"name":"ana"},"body":"earlier","created":"2026-09-30T10:00:00.000+0000"}]},` +
	`"worklog":{"worklogs":[` +
	`{"author":{"name":"ana"},"started":"2026-10-02T11:00:00.000+0000","timeSpent":"1h"}]}},` +
	`"changelog":{"histories":[` +
	`{"author":{"name":"ana"},"created":"2026-10-02T12:00:00.000+0000",` +
	`"items":[{"field":"status","toString":"In Review"}]},` +
	`{"author":{"name":"ben"},"created":"2026-10-02T13:00:00.000+0000","items":[{"field":"status","toString":"Done"}]},` +
	`{"author":{"name":"ana"},"created":"2026-10-02T14:00:00.000+0000","items":[{"field":"labels","toString":"x"}]}]}}`

// activityJira answers who I am, ana, and a search of the issues given out
// of a total, keeping the search's query.
func activityJira(t *testing.T, total int, issues ...string) (jira.Client, *atomic.Value) {
	t.Helper()

	var query atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", jsonMediaType)

		if strings.HasSuffix(request.URL.Path, "/myself") {
			_, _ = writer.Write([]byte(`{"name":"ana","displayName":"Ana Lopez"}`))

			return
		}

		query.Store(request.URL.Query())

		_, _ = writer.Write([]byte(`{"total":` + strconv.Itoa(total) + `,"issues":[` + strings.Join(issues, ",") + `]}`))
	})

	return client, &query
}

func TestActivityReadsWhatYouDidOnEachIssueInThePeriod(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := activityJira(t, 1, activityIssue)
	start := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)

	// Act
	activity, err := client.Activity(t.Context(), start, start.Add(24*time.Hour))

	// Assert
	want := []jira.Event{
		{At: start.Add(9 * time.Hour), Kind: jira.EventCreated, Key: leakKey, Summary: leakSummary},
		{At: start.Add(10 * time.Hour), Kind: jira.EventCommented, Key: leakKey, Summary: leakSummary},
		{At: start.Add(11 * time.Hour), Kind: jira.EventWorked, Key: leakKey, Summary: leakSummary, Detail: "1h"},
		{
			At: start.Add(12 * time.Hour), Kind: jira.EventMoved, Key: leakKey, Summary: leakSummary,
			Detail: "In Review",
		},
	}
	if err != nil || activity.Truncated || len(activity.Events) != len(want) {
		t.Fatalf("Activity = %+v, %v; want %d events, all of them", activity, err, len(want))
	}

	for index, event := range activity.Events {
		if event != want[index] {
			t.Errorf("event %d = %+v, want %+v", index, event, want[index])
		}
	}
}

func TestActivityAsksForTheIssuesYouTouchedWithTheirChangelog(t *testing.T) {
	t.Parallel()

	// Arrange
	client, query := activityJira(t, 0)
	start := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)

	// Act
	_, err := client.Activity(t.Context(), start, start.Add(24*time.Hour))

	// Assert
	asked, _ := query.Load().(url.Values)
	jql := strings.Join(asked["jql"], "")

	// Jira reads a JQL date in the user's own zone, so a day before is asked
	// for and each event is kept by its own time.
	if err != nil || strings.Join(asked["expand"], "") != "changelog" ||
		!strings.Contains(jql, `updated >= "2026/10/01 00:00"`) || !strings.Contains(jql, "status CHANGED BY currentUser()") {
		t.Errorf("asked %v (%v), want the changelog of the issues touched since the day before", asked, err)
	}
}

func TestActivityReadsTheIssuesTouchedEarliestFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	// Only so many pages are read; for a period some way back, the issues
	// updated most recently are the ones least likely to be in it.
	client, query := activityJira(t, 0)
	start := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)

	// Act
	_, err := client.Activity(t.Context(), start, start.Add(24*time.Hour))

	// Assert
	asked, _ := query.Load().(url.Values)
	if jql := strings.Join(asked["jql"], ""); err != nil || !strings.HasSuffix(jql, "ORDER BY updated ASC") {
		t.Errorf("asked %q (%v), want the issues ordered earliest updated first", jql, err)
	}
}

func TestActivitySaysWhenThereWereMoreIssuesThanItRead(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := activityJira(t, 500, activityIssue)
	start := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)

	// Act
	activity, err := client.Activity(t.Context(), start, start.Add(24*time.Hour))

	// Assert
	if err != nil || !activity.Truncated {
		t.Errorf("Activity = %+v, %v; want it marked as having more", activity, err)
	}
}
