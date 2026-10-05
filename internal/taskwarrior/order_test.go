// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior_test

import (
	"slices"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// tagCI and tagWeb are tags the order tests sort by.
const (
	tagCI  = "ci"
	tagWeb = "web"
)

// orderNow is the moment the state order is judged at.
func orderNow() time.Time {
	return time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
}

// uuids is the tasks' uuids in order, which is what an order is judged by.
func uuids(tasks []taskwarrior.Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.UUID)
	}

	return ids
}

func TestEachOrderPutsTasksWhereItsKeySays(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		order func([]taskwarrior.Task) []taskwarrior.Task
		tasks []taskwarrior.Task
		want  []string
	}{
		"state: started, then pending, then waiting": {
			order: func(tasks []taskwarrior.Task) []taskwarrior.Task { return taskwarrior.ByState(tasks, orderNow()) },
			tasks: []taskwarrior.Task{
				{UUID: "s3", Status: taskwarrior.Pending, Wait: orderNow().Add(time.Hour), Urgency: 9},
				{UUID: "s2", Status: taskwarrior.Pending, Urgency: 1},
				{UUID: "s1", Status: taskwarrior.Pending, Start: orderNow().Add(-time.Hour)},
			},
			want: []string{"s1", "s2", "s3"},
		},
		"id: lowest first, and a task outside the working set last": {
			order: taskwarrior.ByID,
			tasks: []taskwarrior.Task{{UUID: "i3", ID: 0}, {UUID: "i2", ID: 12}, {UUID: "i1", ID: 3}},
			want:  []string{"i1", "i2", "i3"},
		},
		"tag: by the tags in order, whatever order they were added in, untagged last": {
			order: taskwarrior.ByTag,
			tasks: []taskwarrior.Task{
				{UUID: "t4"},
				{UUID: "t3", Tags: []string{tagWeb, tagCI}},
				{UUID: "t2", Tags: []string{tagCI}},
				{UUID: "t1", Tags: []string{"api"}},
			},
			want: []string{"t1", "t2", "t3", "t4"},
		},
		"issue: keys in natural order, so PROJ-2 before PROJ-10, and unlinked last": {
			order: taskwarrior.ByIssue,
			tasks: []taskwarrior.Task{
				{UUID: "unlinked"},
				{UUID: "ten", IssueKey: "PROJ-10"},
				{UUID: "two", IssueKey: "PROJ-2"},
				{UUID: "abc", IssueKey: "ABC-7"},
			},
			want: []string{"abc", "two", "ten", "unlinked"},
		},
		"issue: keys equal but for leading zeros fall back to most urgent first": {
			order: taskwarrior.ByIssue,
			tasks: []taskwarrior.Task{
				{UUID: "z2", IssueKey: "PROJ-1", Urgency: 1},
				{UUID: "z1", IssueKey: "PROJ-01", Urgency: 5},
			},
			want: []string{"z1", "z2"},
		},
		"priority: high, medium, low, anything else, then none": {
			order: taskwarrior.ByPriority,
			tasks: []taskwarrior.Task{
				{UUID: "p5"},
				{UUID: "p3", Priority: "L"},
				{UUID: "p4", Priority: "X"},
				{UUID: "p1", Priority: "H"},
				{UUID: "p2", Priority: "M"},
			},
			want: []string{"p1", "p2", "p3", "p4", "p5"},
		},
		"a tie falls back to most urgent first": {
			order: taskwarrior.ByPriority,
			tasks: []taskwarrior.Task{
				{UUID: "u2", Priority: "H", Urgency: 2}, {UUID: "u1", Priority: "H", Urgency: 8},
			},
			want: []string{"u1", "u2"},
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := uuids(testCase.order(testCase.tasks))

			// Assert
			if !slices.Equal(got, testCase.want) {
				t.Errorf("order = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestOrderingLeavesTheTasksItWasGivenAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	// Tags share their backing array across copies of a task, so sorting them
	// in place to compare would reorder the caller's tasks' tags.
	tags := []string{tagWeb, tagCI}
	tasks := []taskwarrior.Task{{UUID: "b", Tags: tags}, {UUID: "a", Tags: []string{"docs"}}}

	// Act
	taskwarrior.ByTag(tasks)

	// Assert
	if !slices.Equal(uuids(tasks), []string{"b", "a"}) || !slices.Equal(tags, []string{tagWeb, tagCI}) {
		t.Errorf("ByTag changed what it was given: tasks %v, tags %v", uuids(tasks), tags)
	}
}

func TestATaskStateIsWordedAsTheListShowsIt(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		task taskwarrior.Task
		want string
	}{
		"a started task":      {taskwarrior.Task{Status: taskwarrior.Pending, Start: orderNow()}, "started"},
		"a pending task":      {taskwarrior.Task{Status: taskwarrior.Pending}, string(taskwarrior.Pending)},
		"a waiting task":      {taskwarrior.Task{Status: taskwarrior.Waiting}, string(taskwarrior.Waiting)},
		"a completed task":    {taskwarrior.Task{Status: taskwarrior.Completed}, string(taskwarrior.Completed)},
		"a status unheard of": {taskwarrior.Task{Status: "invented"}, "unknown"},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := testCase.task.State(orderNow()).String()

			// Assert
			if got != testCase.want {
				t.Errorf("State = %q, want %q", got, testCase.want)
			}
		})
	}
}
