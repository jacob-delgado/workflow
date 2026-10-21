// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// workedKey is the issue the demo works on.
const workedKey jira.Key = "PROJ-412"

// workedSummary is what the worked issue asks for.
const workedSummary = "Redact tokens in the request log"

// The statuses the demo's issues move through, and the people on them.
const (
	statusToDo       = "To Do"
	statusInProgress = "In Progress"
	statusInReview   = "In Review"
	statusDone       = "Done"
	reporter         = "Ana Lopez"
	reviewer         = "Sam Ortiz"
	typeBug          = "Bug"
	typeStory        = "Story"
	typeTask         = "Task"
)

// issueList is the issues the demo's view lists, as the web's mock lists
// them (mockSnapshot.ts), but with PROJ-412 at To Do, not yet started.
func issueList() []jira.Issue {
	return []jira.Issue{
		notStarted(workedKey, workedSummary, typeBug, "High"),
		{
			Key: "PROJ-418", Summary: "Refuse to start when the config names an unknown forge", Status: statusInReview,
			StatusCategory: jira.CategoryIndeterminate, Type: typeBug, Priority: "High",
		},
		notStarted("PROJ-408", "Cache the forge CI status between polls", typeStory, "Medium"),
		notStarted("PROJ-401", "Slugify the issue summary into the branch name", typeTask, ""),
		{
			Key: "PROJ-396", Summary: "Support GitLab merge requests alongside GitHub pulls", Status: "Backlog",
			StatusCategory: jira.CategoryNew, Type: typeStory, Priority: "Medium",
		},
		inProgress("PROJ-390", "Flake in the CI-polling test under the race detector", typeBug, "Low"),
		{
			Key: "PROJ-377", Summary: "Document the on-prem Jira token flow", Status: statusInReview,
			StatusCategory: jira.CategoryIndeterminate, Type: typeTask, Priority: "Low",
		},
	}
}

// inProgress is an issue being worked on.
func inProgress(key jira.Key, summary, kind, priority string) jira.Issue {
	return jira.Issue{
		Key: key, Summary: summary, Status: statusInProgress, StatusCategory: jira.CategoryIndeterminate,
		Type: kind, Priority: priority,
	}
}

// notStarted is an issue no one has started.
func notStarted(key jira.Key, summary, kind, priority string) jira.Issue {
	return jira.Issue{
		Key: key, Summary: summary, Status: statusToDo, StatusCategory: jira.CategoryNew, Type: kind, Priority: priority,
	}
}

// jiraDeps is the demo's Jira.
func (w *World) jiraDeps() seams.Jira {
	return seams.Jira{
		Search:    w.search,
		Issue:     w.issue,
		BrowseURL: browseURL,
		Activity:  w.jiraActivity,
		// What the demo does to its issues.
		Transitions: w.transitions,
		Transition:  w.transition,
	}
}

// workflow is the moves each status offers, as the demo's Jira project
// allows them: on from to do to in progress, review and done.
func workflow() map[string][]jira.Transition {
	return map[string][]jira.Transition{
		statusToDo: {{
			ID: "11", Name: "Start Progress", ToStatus: statusInProgress,
			ToStatusCategory: jira.CategoryIndeterminate,
		}},
		statusInProgress: {{
			ID: "21", Name: "Request Review", ToStatus: statusInReview,
			ToStatusCategory: jira.CategoryIndeterminate,
		}},
		statusInReview: {{ID: "31", Name: "Done", ToStatus: statusDone, ToStatusCategory: jira.CategoryDone}},
	}
}

// transitions is the moves an issue's status offers.
func (w *World) transitions(_ context.Context, key jira.Key) ([]jira.Transition, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	index, err := w.indexOf(key)
	if err != nil {
		return nil, err
	}

	return workflow()[w.issues[index].Status], nil
}

// transition moves an issue by a move its status offers, refusing any other
// as Jira does.
func (w *World) transition(_ context.Context, key jira.Key, move jira.Transition, _ []jira.FieldValue) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	index, err := w.indexOf(key)
	if err != nil {
		return err
	}

	issue := w.issues[index]

	offered := slices.ContainsFunc(workflow()[issue.Status], func(each jira.Transition) bool { return each.ID == move.ID })
	if !offered {
		return fmt.Errorf("%s cannot move by %q from %s: %w", key, move.Name, issue.Status, jira.ErrRejected)
	}

	issue.Status, issue.StatusCategory = move.ToStatus, move.ToStatusCategory
	w.issues[index] = issue
	w.moves = append(w.moves,
		jira.Event{At: w.now(), Kind: jira.EventMoved, Key: key, Summary: issue.Summary, Detail: move.ToStatus})

	return nil
}

// search lists every issue, a page at a time, whatever the view asks for:
// the demo has one view's worth of issues.
func (w *World) search(_ context.Context, _ string, startAt int) (jira.SearchResult, error) {
	return locked(w, func() jira.SearchResult {
		start := min(max(startAt, 0), len(w.issues))

		return jira.SearchResult{Issues: slices.Clone(w.issues[start:]), Total: len(w.issues)}
	}), nil
}

// issue reads an issue in full: the issue as it stands, and what was written
// on it. PROJ-412 carries the web mock's words (mockIssues.ts); an issue the
// demo does not list is not found, as Jira answers one.
func (w *World) issue(_ context.Context, key jira.Key) (jira.IssueDetail, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	index, err := w.indexOf(key)
	if err != nil {
		return jira.IssueDetail{}, err
	}

	detail := issueNotes()[key]
	detail.Issue, detail.Reporter, detail.Assignee = w.issues[index], reporter, reporter

	return detail, nil
}

// issueNotes is what was written on the demo's issues, by key; an issue with
// none has an empty description and no comments.
func issueNotes() map[jira.Key]jira.IssueDetail {
	return map[jira.Key]jira.IssueDetail{
		workedKey: {
			Description: "The request log records every header, so a bearer token lands in the log file.\n\n" +
				"Redact the Authorization header before the line is written.",
			Comments: []jira.Comment{{
				Author: reviewer, Body: "Repro'd on main with --log; the token is in the third line.",
				Created: epoch().Add(-2 * time.Hour),
			}},
			CommentTotal: 1,
		},
		"PROJ-408": {Description: "Each poll asks the forge again; keep the last answer for the interval."},
	}
}

// indexOf is where an issue stands in the list, or not found, as Jira
// answers an issue it does not have. The caller holds the world's lock.
func (w *World) indexOf(key jira.Key) (int, error) {
	index, listed := w.position[key]
	if !listed {
		return 0, fmt.Errorf("issue %s: %w", key, jira.ErrNotFound)
	}

	return index, nil
}

// browseURL links an issue for someone to click.
func browseURL(key jira.Key) string {
	return jiraBase + "/browse/" + string(key)
}
