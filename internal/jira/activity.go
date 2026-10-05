// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"cmp"
	"context"
	"net/http"
	"slices"
	"strings"
	"time"
)

// activityFields are the fields Activity reads of each issue: enough to name
// it, and the comments and worklogs it reads your own from.
const activityFields = "summary,created,reporter,comment,worklog"

// activityPages bounds how many pages of issues Activity reads, so a long
// period on a busy instance answers with what it has rather than walking it.
const activityPages = 2

// jqlDateLayout is how JQL writes a date: Jira reads it in the user's zone.
const jqlDateLayout = "2006/01/02 15:04"

// activityJQL finds the issues you touched since a date: reported, assigned,
// worked on or moved, the earliest updated first, since a period some way back
// is likelier among them than among the latest. A comment you left on an
// issue you did nothing else to is not found, since JQL has no way to ask for
// one.
const activityJQL = `updated >= "%s" AND (reporter = currentUser() OR assignee was currentUser() ` +
	`OR worklogAuthor = currentUser() OR status CHANGED BY currentUser()) ORDER BY updated ASC`

// EventKind is what you did to an issue.
type EventKind int

const (
	// EventCreated is an issue you reported.
	EventCreated EventKind = iota + 1
	// EventMoved is an issue you moved to another status, named in Detail.
	EventMoved
	// EventWorked is work you logged, its time spent in Detail.
	EventWorked
	// EventCommented is a comment you wrote.
	EventCommented
)

// Event is one thing you did to an issue, and when.
type Event struct {
	At      time.Time
	Kind    EventKind
	Key     Key
	Summary string
	Detail  string
}

// Activity is what you did to issues over a period, oldest first, and
// whether there were more issues touched than were read.
type Activity struct {
	Events    []Event
	Truncated bool
}

// Activity reads what you did to issues from start up to end. Jira reads a
// JQL date in the user's own zone, so the issues updated since the day before
// start are read, and each event is kept by its own time.
func (c Client) Activity(ctx context.Context, start, end time.Time) (Activity, error) {
	myself, err := c.Myself(ctx)
	if err != nil {
		return Activity{}, err
	}

	issues, truncated, err := c.touchedIssues(ctx, start.Add(-24*time.Hour))
	if err != nil {
		return Activity{}, err
	}

	var events []Event

	for _, issue := range issues {
		for _, event := range issue.events(myself.Name) {
			if !event.At.Before(start) && event.At.Before(end) {
				events = append(events, event)
			}
		}
	}

	slices.SortStableFunc(events, func(a, b Event) int {
		return cmp.Or(a.At.Compare(b.At), cmp.Compare(a.Kind, b.Kind))
	})

	return Activity{Events: events, Truncated: truncated}, nil
}

// touchedIssues reads the issues you touched since a time, a page at a time
// up to activityPages, and whether Jira counted more than were read.
func (c Client) touchedIssues(ctx context.Context, since time.Time) ([]wireActivityIssue, bool, error) {
	jql := strings.Replace(activityJQL, "%s", since.UTC().Format(jqlDateLayout), 1)

	var issues []wireActivityIssue

	for page := range activityPages {
		query := searchQuery(jql, page*searchLimit)
		query.Set("fields", activityFields)
		query.Set("expand", "changelog")

		request, err := c.newRequest(ctx, http.MethodGet, searchPath+"?"+query.Encode(), nil)

		answer, err := decode[wireActivityAnswer](c, request, err)
		if err != nil {
			return nil, false, err
		}

		issues = append(issues, answer.Issues...)
		if len(answer.Issues) < searchLimit || len(issues) >= answer.Total {
			return issues, len(issues) < answer.Total, nil
		}
	}

	return issues, true, nil
}

// wireActivityAnswer is a changelog-expanded search answer.
type wireActivityAnswer struct {
	Total  int                 `json:"total"`
	Issues []wireActivityIssue `json:"issues"`
}

// login is a user as Activity tells them apart: by login, which an instance
// never withholds.
type login struct {
	Name string `json:"name"`
}

// wireActivityIssue is an issue as Activity reads it.
type wireActivityIssue struct {
	Key    Key `json:"key"`
	Fields struct {
		Summary  string `json:"summary"`
		Created  string `json:"created"`
		Reporter login  `json:"reporter"`
		Comment  struct {
			Comments []struct {
				Author  login  `json:"author"`
				Created string `json:"created"`
			} `json:"comments"`
		} `json:"comment"`
		Worklog struct {
			Worklogs []struct {
				Author    login  `json:"author"`
				Started   string `json:"started"`
				TimeSpent string `json:"timeSpent"` //nolint:tagliatelle // Jira's field name on the wire, not ours to pick
			} `json:"worklogs"`
		} `json:"worklog"`
	} `json:"fields"`
	Changelog struct {
		Histories []struct {
			Author  login  `json:"author"`
			Created string `json:"created"`
			Items   []struct {
				Field    string `json:"field"`
				ToString string `json:"toString"` //nolint:tagliatelle // Jira's field name on the wire, not ours to pick
			} `json:"items"`
		} `json:"histories"`
	} `json:"changelog"`
}

// events are what me did to the issue, each at a time Jira wrote in a form it
// can be read in.
func (w wireActivityIssue) events(me string) []Event {
	var events []Event

	add := func(by, at string, kind EventKind, detail string) {
		when, err := time.Parse(timeLayout, at)
		if by == me && err == nil {
			events = append(events, Event{At: when.UTC(), Kind: kind, Key: w.Key, Summary: w.Fields.Summary, Detail: detail})
		}
	}

	add(w.Fields.Reporter.Name, w.Fields.Created, EventCreated, "")

	for _, comment := range w.Fields.Comment.Comments {
		add(comment.Author.Name, comment.Created, EventCommented, "")
	}

	for _, worklog := range w.Fields.Worklog.Worklogs {
		add(worklog.Author.Name, worklog.Started, EventWorked, worklog.TimeSpent)
	}

	for _, history := range w.Changelog.Histories {
		for _, item := range history.Items {
			if item.Field == "status" {
				add(history.Author.Name, history.Created, EventMoved, item.ToString)
			}
		}
	}

	return events
}
