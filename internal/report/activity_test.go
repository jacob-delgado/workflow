// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package report_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/report"
)

// errRefused is a source that is set up and refused.
var errRefused = errors.New("refused")

// tuesday is the day the summaries here are of, and wednesday the day after,
// when they are read.
func tuesday() activity.Date {
	return activity.DateOf(time.Date(2026, time.March, 3, 12, 0, 0, 0, time.UTC))
}

func wednesday() activity.Date {
	return activity.DateOf(time.Date(2026, time.March, 4, 12, 0, 0, 0, time.UTC))
}

// worded is a describe that names the failure it was handed.
func worded(err error) string { return "worded: " + err.Error() }

// summaryOf is a summary of tuesday's reads.
func summaryOf(reads ...activity.Read) activity.Summary {
	return activity.Summary{Period: activity.Period{From: tuesday(), To: tuesday()}, Reads: reads}
}

// sourceIn is the source of the report named name.
func sourceIn(t *testing.T, answer api.Activity, name api.ActivitySourceName) api.ActivitySource {
	t.Helper()

	for _, source := range answer.Sources {
		if source.Source == name {
			return source
		}
	}

	t.Fatalf("the report has no %s source: %+v", name, answer.Sources)

	return api.ActivitySource{}
}

func TestTheActivityReportNestsEachItemByYearMonthDayAndHour(t *testing.T) {
	t.Parallel()

	// Arrange
	committedAt := time.Date(2026, time.March, 3, 9, 30, 0, 0, time.UTC)
	committed := activity.Item{
		At: committedAt, Kind: activity.Committed, Ref: "abc1234", Title: "fix: redact tokens", Repository: "workflow",
	}
	summary := summaryOf(activity.Read{Source: activity.SourceGit, Items: []activity.Item{committed}})

	// Act
	answer := report.Activity(summary, wednesday(), time.UTC, worded)

	// Assert
	item := api.ActivityItem{
		At: committedAt, Source: api.ActivitySourceNameGit, Verb: "committed", Ref: "abc1234", Title: "fix: redact tokens",
		URL: "", Repository: "workflow",
	}
	hours := activity.Group([]activity.Item{committed}, time.UTC)[0].Months[0].Days[0].Hours
	want := []api.ActivityYear{{Year: 2026, Months: []api.ActivityMonth{{
		Month: 3, Name: "March", Days: []api.ActivityDay{{
			Date: tuesday().String(), Weekday: "Tuesday",
			Hours: []api.ActivityHour{{Label: hours[0].Label, Items: []api.ActivityItem{item}}},
		}},
	}}}}

	if !reflect.DeepEqual(answer.Years, want) {
		t.Errorf("Years = %+v, want the commit under 2026, March, Tuesday the 3rd, at its hour: %+v", answer.Years, want)
	}
}

func TestTheActivityReportSaysThePeriodAndToday(t *testing.T) {
	t.Parallel()

	// Act
	answer := report.Activity(summaryOf(), wednesday(), time.UTC, worded)

	// Assert
	if answer.From.String() != tuesday().String() || answer.To.String() != tuesday().String() ||
		answer.Today.String() != wednesday().String() {
		t.Errorf("the report runs %s to %s, today %s; want Tuesday, read on Wednesday",
			answer.From, answer.To, answer.Today)
	}
}

func TestTheActivityReportNamesEachSourceAsTheAPIDoes(t *testing.T) {
	t.Parallel()

	sources := map[activity.Source]api.ActivitySourceName{
		activity.SourceGit: api.ActivitySourceNameGit, activity.SourceTasks: api.ActivitySourceNameTasks,
		activity.SourceJira: api.ActivitySourceNameJira, activity.SourceForge: api.ActivitySourceNameForge,
	}

	for source, want := range sources {
		t.Run(source.Name(), func(t *testing.T) {
			t.Parallel()

			// Act
			answer := report.Activity(summaryOf(activity.Read{Source: source}), wednesday(), time.UTC, worded)

			// Assert
			if got := answer.Sources[0]; got.Source != want || got.State != api.ActivitySourceStateRead {
				t.Errorf("source %s reads as %q, %q; want %q, read", source.Name(), got.Source, got.State, want)
			}
		})
	}
}

func TestAnUnknownSourceIsNamedAsGit(t *testing.T) {
	t.Parallel()

	// Act
	answer := report.Activity(summaryOf(activity.Read{Source: 0}), wednesday(), time.UTC, worded)

	// Assert
	if got := answer.Sources[0].Source; got != api.ActivitySourceNameGit {
		t.Errorf("an unknown source reads as %q, want %q", got, api.ActivitySourceNameGit)
	}
}

func TestASourceThatFailedSaysWhyInDescribesWords(t *testing.T) {
	t.Parallel()

	// Arrange
	summary := summaryOf(activity.Read{Source: activity.SourceJira, Failed: errRefused})

	// Act
	answer := report.Activity(summary, wednesday(), time.UTC, worded)

	// Assert
	source := sourceIn(t, answer, api.ActivitySourceNameJira)
	if source.State != api.ActivitySourceStateFailed || source.Detail != "worded: refused" {
		t.Errorf("Jira = %q, %q; want failed, in describe's words", source.State, source.Detail)
	}
}

func TestASourceNotSetUpSaysHowInDescribesWords(t *testing.T) {
	t.Parallel()

	// Arrange
	summary := summaryOf(activity.Read{Source: activity.SourceTasks, NotSetUp: errRefused})

	// Act
	answer := report.Activity(summary, wednesday(), time.UTC, worded)

	// Assert
	source := sourceIn(t, answer, api.ActivitySourceNameTasks)
	if source.State != api.ActivitySourceStateNotSetUp || source.Detail != "worded: refused" {
		t.Errorf("Taskwarrior = %q, %q; want not set up, in describe's words", source.State, source.Detail)
	}
}

func TestAFailureInOneOfSeveralRepositoriesNamesIt(t *testing.T) {
	t.Parallel()

	// Arrange
	failed := loop.RepositoryErrors{
		loop.RepositoryError{Repository: "api", Err: errRefused},
		loop.RepositoryError{Repository: "gone", Err: gitrepo.ErrNotARepository},
	}
	summary := summaryOf(activity.Read{Source: activity.SourceGit, Failed: failed})

	// Act
	answer := report.Activity(summary, wednesday(), time.UTC, worded)

	// Assert
	want := "api: worded: refused; gone is no longer a git repository"
	if got := sourceIn(t, answer, api.ActivitySourceNameGit).Detail; got != want {
		t.Errorf("git's detail = %q, want %q", got, want)
	}
}

func TestThePostLengthIsCountedAgainstTheServicesLimit(t *testing.T) {
	t.Parallel()

	// Arrange
	slack := config.Messaging{Kind: config.KindSlack, WebhookURL: "https://hooks.slack.example/T0/B0/x"}

	// Act
	length := report.PostLength(slack, "shipped")

	// Assert
	if length == nil || length.Count != len("shipped") || length.Limit == 0 {
		t.Errorf("PostLength = %+v, want the seven characters against Slack's limit", length)
	}
}

func TestThereIsNoPostLengthWithNothingToPostTo(t *testing.T) {
	t.Parallel()

	// Act
	length := report.PostLength(config.Messaging{}, "shipped")

	// Assert
	if length != nil {
		t.Errorf("PostLength with no messaging = %+v, want none", length)
	}
}

func TestThereIsNoPostLengthForAServiceThatNamesNoLimit(t *testing.T) {
	t.Parallel()

	// Arrange
	webhook := config.Messaging{Kind: config.KindWebhook, WebhookURL: "https://hooks.example/x"}

	// Act
	length := report.PostLength(webhook, "shipped")

	// Assert
	if length != nil {
		t.Errorf("PostLength through a plain webhook = %+v, want none", length)
	}
}
