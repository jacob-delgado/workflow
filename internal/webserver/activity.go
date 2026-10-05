// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"time"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// GetActivity reads back what you did over the period asked, or the previous
// working day, in the server's own time zone: each source on its own, so one
// that cannot be read is named and the rest still answer.
func (s *server) GetActivity(
	_ context.Context, request api.GetActivityRequestObject,
) (api.GetActivityResponseObject, error) {
	now := s.now()
	today := activity.DateOf(now)

	period, err := periodAsked(request.Params, today)
	if err != nil {
		return periodRefused(err), nil
	}

	start, end := period.Bounds(now.Location())
	summary := activity.Summary{Period: period, Reads: s.activityReads(start, end)}

	return api.GetActivity200JSONResponse(s.activityDTO(summary, today, now.Location())), nil
}

// periodRefused is a period that could not be read as asked, and why, in
// words of its own: a date that could not be read is not repeated back.
func periodRefused(err error) api.GetActivityResponseObject {
	reason := err.Error()
	if errors.Is(err, activity.ErrNotADate) {
		reason = "from and to are each a date written as YYYY-MM-DD"
	}

	return api.GetActivity422ApplicationProblemPlusJSONResponse(
		problem(api.Unprocessable, "the period could not be read: "+reason))
}

// periodAsked is the period from and to name: both days, one day alone, or
// the previous working day when neither is given.
func periodAsked(params api.GetActivityParams, today activity.Date) (activity.Period, error) {
	switch {
	case params.From == nil && params.To == nil:
		return activity.PreviousWorkingDay(today), nil
	case params.From == nil:
		return dayAlone(*params.To)
	case params.To == nil:
		return dayAlone(*params.From)
	}

	from, err := activity.ParseDate(*params.From)
	if err != nil {
		return activity.Period{}, err
	}

	to, err := activity.ParseDate(*params.To)
	if err != nil {
		return activity.Period{}, err
	}

	return activity.NewPeriod(from, to)
}

// dayAlone is the period of one day, written year-month-day.
func dayAlone(text string) (activity.Period, error) {
	day, err := activity.ParseDate(text)

	return activity.Period{From: day, To: day}, err
}

// activityReads asks every source the server reaches, one after another.
func (s *server) activityReads(start, end time.Time) []activity.Read {
	var reads []activity.Read

	if s.deps.CommitsBetween != nil {
		reads = append(reads, loop.CommitsRead(s.deps.CommitsBetween, start, end))
	}

	if s.deps.Tasks.Touched != nil {
		reads = append(reads, loop.TasksRead(s.deps.Tasks.Touched, start, end))
	}

	if s.deps.JiraActivity != nil {
		browse := s.deps.BrowseURL
		if browse == nil {
			browse = func(jira.Key) string { return "" }
		}

		reads = append(reads, loop.JiraRead(s.deps.JiraActivity, browse, start, end))
	}

	if s.deps.ForgeActivity != nil {
		reads = append(reads, loop.ForgeRead(s.deps.ForgeActivity, s.forgeKindNow(), start, end))
	}

	return reads
}

// activityDTO is the summary as the API answers it. A source's failure is
// worded as fault words it, so it never names a host.
func (s *server) activityDTO(summary activity.Summary, today activity.Date, loc *time.Location) api.Activity {
	sources := make([]api.ActivitySource, 0, len(summary.Reads))

	for _, read := range summary.Reads {
		source := api.ActivitySource{
			Source: sourceName(read.Source), Name: read.Source.Title(), Failed: read.Failed != nil,
			Truncated: read.Truncated, Detail: "",
		}
		if read.Failed != nil {
			body, _ := s.fault(read.Failed)
			source.Detail = body.Detail
		}

		sources = append(sources, source)
	}

	return api.Activity{
		From: summary.Period.From.String(), To: summary.Period.To.String(), Today: today.String(),
		Sources: sources, Years: yearsDTO(activity.Group(summary.Items(), loc)), Text: summary.Text(loc),
	}
}

// yearsDTO is the grouping as the API nests it.
func yearsDTO(years []activity.Year) []api.ActivityYear {
	nested := make([]api.ActivityYear, 0, len(years))

	for _, year := range years {
		months := make([]api.ActivityMonth, 0, len(year.Months))

		for _, month := range year.Months {
			days := make([]api.ActivityDay, 0, len(month.Days))

			for _, day := range month.Days {
				days = append(days, api.ActivityDay{
					Date: day.Date.String(), Weekday: day.Date.Weekday().String(), Hours: hoursDTO(day.Hours),
				})
			}

			months = append(months, api.ActivityMonth{Month: int(month.Month), Name: month.Month.String(), Days: days})
		}

		nested = append(nested, api.ActivityYear{Year: year.Year, Months: months})
	}

	return nested
}

// hoursDTO is a day's hours and their items as the API answers them.
func hoursDTO(hours []activity.Hour) []api.ActivityHour {
	shown := make([]api.ActivityHour, 0, len(hours))

	for _, hour := range hours {
		items := make([]api.ActivityItem, 0, len(hour.Items))
		for _, item := range hour.Items {
			items = append(items, api.ActivityItem{
				At: item.At, Source: sourceName(item.Kind.Source()), Verb: item.Kind.Verb(),
				Ref: item.Ref, Title: item.Title, URL: item.URL, Repository: item.Repository,
			})
		}

		shown = append(shown, api.ActivityHour{Label: hour.Label, Items: items})
	}

	return shown
}

// sourceName is a source as the API names it.
func sourceName(source activity.Source) api.ActivitySourceName {
	switch source {
	case activity.SourceGit:
		return api.ActivitySourceNameGit
	case activity.SourceTasks:
		return api.ActivitySourceNameTasks
	case activity.SourceJira:
		return api.ActivitySourceNameJira
	case activity.SourceForge:
		return api.ActivitySourceNameForge
	}

	return api.ActivitySourceNameGit
}
