// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// issueKey is the Jira issue the tasks in these tests link to.
const issueKey = "PROJ-42"

// lineSeparator is U+2028, which JSON carries unescaped and a terminal breaks
// the line at.
const lineSeparator = string(rune(0x2028))

// everyField is one task with every field export writes, as export writes it.
const everyField = `[{
	"id": 12,
	"uuid": "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
	"description": "Renew the cert",
	"status": "pending",
	"project": "ops",
	"priority": "H",
	"tags": ["cert", "jira"],
	"due": "20260929T000000Z",
	"wait": "20260920T080000Z",
	"scheduled": "20260921T090000Z",
	"until": "20261001T100000Z",
	"entry": "20260901T101500Z",
	"modified": "20260902T111600Z",
	"start": "20260903T121700Z",
	"end": "20260904T131800Z",
	"urgency": 14.2,
	"annotations": [{"entry": "20260905T141900Z", "description": "see #42"}],
	"depends": ["b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e"],
	"jiraid": "PROJ-42",
	"jiraurl": "https://jira.example.com/browse/PROJ-42",
	"jirasummary": "Renew the cert",
	"recur": "weekly"
}]
`

func TestATaskDecodesEveryFieldTaskwarriorWrites(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := answering(exportWord, everyField)
	want := taskwarrior.Task{
		UUID:        "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
		ID:          12,
		Description: "Renew the cert",
		Status:      taskwarrior.Pending,
		Project:     "ops",
		Priority:    "H",
		Tags:        []string{"cert", "jira"},
		Due:         time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC),
		Wait:        time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC),
		Scheduled:   time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC),
		Until:       time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC),
		Entry:       time.Date(2026, 9, 1, 10, 15, 0, 0, time.UTC),
		Modified:    time.Date(2026, 9, 2, 11, 16, 0, 0, time.UTC),
		Start:       time.Date(2026, 9, 3, 12, 17, 0, 0, time.UTC),
		End:         time.Date(2026, 9, 4, 13, 18, 0, 0, time.UTC),
		Urgency:     14.2,
		Annotations: []taskwarrior.Annotation{
			{Entry: time.Date(2026, 9, 5, 14, 19, 0, 0, time.UTC), Description: "see #42"},
		},
		Depends:  []string{"b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e"},
		IssueKey: issueKey,
		IssueURL: "https://jira.example.com/browse/PROJ-42",
	}

	// Act
	tasks, err := fake.client().Linked(t.Context())
	// Assert
	if err != nil {
		t.Fatalf("Linked returned %v", err)
	}

	if len(tasks) != 1 || !reflect.DeepEqual(tasks[0], want) {
		t.Errorf("Linked decoded\n%+v\nwant\n%+v", tasks, want)
	}
}

func TestDependsDecodesAsAnArrayOrALegacyString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		depends string
	}{
		{name: "an array", depends: `["a","b"]`},
		{name: "a legacy comma-joined string", depends: `"a,b"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := answering(exportWord, `[{"uuid":"u","depends":`+test.depends+`}]`)

			// Act
			tasks, err := fake.client().Linked(t.Context())

			// Assert
			if err != nil || len(tasks) != 1 {
				t.Fatalf("Linked = %+v, %v; want one task", tasks, err)
			}

			if want := []string{"a", "b"}; !slices.Equal(tasks[0].Depends, want) {
				t.Errorf("Depends = %q, want %q", tasks[0].Depends, want)
			}

			if tasks[0].Linked() {
				t.Errorf("a task without %s is Linked: %+v", taskwarrior.LinkUDA, tasks[0])
			}
		})
	}
}

func TestTextFieldsAreSanitizedOnTheWayIn(t *testing.T) {
	t.Parallel()

	// Arrange
	const (
		escape  = "\x1b"
		hostile = escape + "[31m" + lineSeparator
	)

	export, err := json.Marshal([]map[string]any{{
		"uuid":        "u",
		"description": "Renew" + hostile,
		"project":     "ops" + hostile,
		"tags":        []string{"cert" + hostile},
		"annotations": []map[string]string{{"entry": "20260905T141900Z", "description": "see" + hostile}},
		"jiraid":      issueKey + hostile,
		"jiraurl":     "https://jira.example.com/browse/PROJ-42" + hostile,
	}})
	if err != nil {
		t.Fatal(err)
	}

	fake := answering(exportWord, string(export))

	// Act
	tasks, err := fake.client().Linked(t.Context())

	// Assert
	if err != nil || len(tasks) != 1 || len(tasks[0].Tags) != 1 || len(tasks[0].Annotations) != 1 {
		t.Fatalf("Linked = %+v, %v; want one task with a tag and an annotation", tasks, err)
	}

	task := tasks[0]
	for _, text := range []string{
		task.Description, task.Project, task.Tags[0], task.Annotations[0].Description, task.IssueKey, task.IssueURL,
	} {
		if strings.ContainsAny(text, escape+lineSeparator) {
			t.Errorf("%q kept a control or a line separator", text)
		}
	}
}

func TestActiveAndWaitingReadTheDates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	waiting := func(task taskwarrior.Task) bool { return task.Waiting(now) }

	tests := []struct {
		name string
		task taskwarrior.Task
		read func(taskwarrior.Task) bool
		want bool
	}{
		{
			name: "a started pending task is active",
			task: taskwarrior.Task{Status: taskwarrior.Pending, Start: now.Add(-time.Hour)},
			read: taskwarrior.Task.Active,
			want: true,
		},
		{
			name: "a pending task never started is not active",
			task: taskwarrior.Task{Status: taskwarrior.Pending},
			read: taskwarrior.Task.Active,
			want: false,
		},
		{
			name: "a completed task that was started is not active",
			task: taskwarrior.Task{Status: taskwarrior.Completed, Start: now.Add(-time.Hour)},
			read: taskwarrior.Task.Active,
			want: false,
		},
		{
			name: "a waiting task is waiting",
			task: taskwarrior.Task{Status: taskwarrior.Waiting},
			read: waiting,
			want: true,
		},
		{
			name: "a pending task waiting until tomorrow is waiting",
			task: taskwarrior.Task{Status: taskwarrior.Pending, Wait: now.AddDate(0, 0, 1)},
			read: waiting,
			want: true,
		},
		{
			name: "a pending task that waited until yesterday is not",
			task: taskwarrior.Task{Status: taskwarrior.Pending, Wait: now.AddDate(0, 0, -1)},
			read: waiting,
			want: false,
		},
		{
			name: "a completed task waiting until tomorrow is not",
			task: taskwarrior.Task{Status: taskwarrior.Completed, Wait: now.AddDate(0, 0, 1)},
			read: waiting,
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := test.read(test.task); got != test.want {
				t.Errorf("got %t, want %t", got, test.want)
			}
		})
	}
}

func TestPriorityForMapsJirasNamesOntoHML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		jira string
		want string
	}{
		{jira: "Highest", want: "H"},
		{jira: "HIGH", want: "H"},
		{jira: "critical", want: "H"},
		{jira: "Blocker", want: "H"},
		{jira: "Medium", want: "M"},
		{jira: "MAJOR", want: "M"},
		{jira: "low", want: "L"},
		{jira: "Lowest", want: "L"},
		{jira: "Minor", want: "L"},
		{jira: "TRIVIAL", want: "L"},
		{jira: "Unknown", want: ""},
	}

	for _, test := range tests {
		t.Run(test.jira, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := taskwarrior.PriorityFor(test.jira); got != test.want {
				t.Errorf("PriorityFor(%q) = %q, want %q", test.jira, got, test.want)
			}
		})
	}
}

func TestTrackLinePutsTheDescriptionAfterTheDoubleDash(t *testing.T) {
	t.Parallel()

	const url = "https://jira.example.com/browse/PROJ-42"

	tests := []struct {
		name  string
		issue taskwarrior.IssueLink
		want  string
	}{
		{
			name:  "a known priority",
			issue: taskwarrior.IssueLink{Key: issueKey, Summary: "Fix due:tomorrow +weird", URL: url, Priority: "High"},
			want:  "jiraid:PROJ-42 jiraurl:" + url + " +jira priority:H -- PROJ-42: Fix due:tomorrow +weird",
		},
		{
			name:  "no priority",
			issue: taskwarrior.IssueLink{Key: issueKey, Summary: "Fix it", URL: url},
			want:  "jiraid:PROJ-42 jiraurl:" + url + " +jira -- PROJ-42: Fix it",
		},
		{
			name:  "a summary over two lines",
			issue: taskwarrior.IssueLink{Key: issueKey, Summary: "Fix\nit", URL: url, Priority: "Low"},
			want:  "jiraid:PROJ-42 jiraurl:" + url + " +jira priority:L -- PROJ-42: " + sanitize.Line("Fix\nit"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := taskwarrior.TrackLine(test.issue)

			// Assert
			if got != test.want {
				t.Errorf("TrackLine = %q, want %q", got, test.want)
			}

			if strings.Contains(got, "\n") {
				t.Errorf("TrackLine = %q, want one line", got)
			}
		})
	}
}
