// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
)

// missingTaskwarrior is a configuration naming a task program that does not
// exist, so Taskwarrior reads as not installed whatever is on PATH.
const missingTaskwarrior = `"taskwarrior":{"program":"/nonexistent/workflow-test/task"}`

func TestSummaryLeavesOutASourceThatIsNotSetUp(t *testing.T) {
	// Arrange
	// No origin names a forge, and no Taskwarrior is installed.
	repo := workedRepository(t, "Add the widget")
	writeFile(t, repo, `{`+missingTaskwarrior+`}`)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay)

	// Assert
	if err != nil || !strings.Contains(printed.stdout, "Add the widget") {
		t.Errorf("summary = %v, printing:\n%s\nwant the commit printed and success", err, printed.stdout)
	}

	for _, want := range []string{
		"The forge is not set up, so it was left out: origin does not name a repository on a forge",
		"Taskwarrior is not set up, so it was left out: Taskwarrior is not installed",
	} {
		if !strings.Contains(printed.stderr, want) {
			t.Errorf("stderr = %q, want a note %q", printed.stderr, want)
		}
	}

	if strings.Contains(printed.stdout+printed.stderr, "could not be read") {
		t.Errorf("summary said a source could not be read:\n%s\n%s", printed.stdout, printed.stderr)
	}
}

func TestSummaryFailsWhenAConfiguredForgeRefuses(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{search: "{", userError: true})
	repo := workedRepository(t, "Add the widget")
	git(t, repo, "remote", "add", "origin", "https://github.com/owner/repo.git")
	writeFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"},`+missingTaskwarrior+`}`)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay)

	// Assert
	if !strings.Contains(printed.stdout, "Add the widget") || err == nil ||
		!strings.Contains(err.Error(), "The forge could not be read") {
		t.Errorf("summary = %v, printing:\n%s\nwant the commit printed, then the forge named as unread", err, printed.stdout)
	}
}

func TestSummaryAsJSONMarksASourceNotSetUpApartFromOneThatFailed(t *testing.T) {
	// Arrange
	jira := jiraServer(t, http.StatusUnauthorized, `{}`, new(atomic.Bool))
	repo := workedRepository(t, "Add the widget")
	writeFile(t, repo, `{"jira":{"base_url":"`+jira.URL+`","token":"t"},`+missingTaskwarrior+`}`)

	// Act
	printed, _ := runStreams(t, repo, unusedPrompt(t), "summary", "--from", summaryDay, "--to", summaryDay, asJSON)

	// Assert
	var got api.Activity

	err := json.Unmarshal([]byte(printed.stdout), &got)
	if err != nil {
		t.Fatalf("summary --json printed what is not JSON: %v\n%s", err, printed.stdout)
	}

	states := map[api.ActivitySourceName]api.ActivitySourceState{}
	for _, source := range got.Sources {
		states[source.Source] = source.State
	}

	want := map[api.ActivitySourceName]api.ActivitySourceState{
		api.ActivitySourceNameGit: api.ActivitySourceRead, api.ActivitySourceNameTasks: api.ActivitySourceNotSetUp,
		api.ActivitySourceNameJira: api.ActivitySourceFailed, api.ActivitySourceNameForge: api.ActivitySourceNotSetUp,
	}
	for source, state := range want {
		if states[source] != state {
			t.Errorf("%s is %q, want %q (every state: %v)", source, states[source], state, states)
		}
	}
}
