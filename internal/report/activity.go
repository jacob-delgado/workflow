// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"errors"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// Activity is the summary as the API answers it, today being the day it is
// in loc, so the command line prints the same shape the web reads. Each
// failure is worded by describe, which the web makes its own fault and the
// command line FaultDetail, so it never names a host.
func Activity(
	summary activity.Summary, today activity.Date, loc *time.Location, describe func(error) string,
) api.Activity {
	sources := make([]api.ActivitySource, 0, len(summary.Reads))

	for _, read := range summary.Reads {
		sources = append(sources, sourceDTO(read, describe))
	}

	return api.Activity{
		From: Date(summary.Period.From), To: Date(summary.Period.To), Today: Date(today),
		Sources: sources, Years: yearsDTO(activity.Group(summary.Items(), loc)), Text: summary.Text(loc),
	}
}

// PostLength is how long text would be posted through settings, against the
// most its service takes, or nil where nothing is set up to post to or the
// service names no limit.
func PostLength(settings config.Messaging, text string) *api.PostLength {
	if settings.Mode() == config.MessagingNone {
		return nil
	}

	length := loop.SummaryLength(settings.Kind, text)
	if length.Limit == 0 {
		return nil
	}

	return &api.PostLength{
		Service: length.Service, Count: length.Count, Limit: length.Limit, Unit: api.PostLengthUnit(length.Unit),
	}
}

// Date is a day as the contract writes a date.
func Date(day activity.Date) openapi_types.Date {
	return openapi_types.Date{Time: time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)}
}

// sourceDTO is how one source answered, as the API says it: read, left out
// as not set up with how to set it up, or failed with why, each worded by
// describe.
func sourceDTO(read activity.Read, describe func(error) string) api.ActivitySource {
	source := api.ActivitySource{
		Source: sourceName(read.Source), Name: read.Source.Title(), State: api.ActivitySourceStateRead,
		Truncated: read.Truncated, Detail: "",
	}

	switch {
	case read.Failed != nil:
		source.State, source.Detail = api.ActivitySourceStateFailed, failureDetail(read.Failed, describe)
	case read.NotSetUp != nil:
		source.State, source.Detail = api.ActivitySourceStateNotSetUp, failureDetail(read.NotSetUp, describe)
	}

	return source
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

// failureDetail words why a source could not be read as describe words each
// of its failures, naming the repository a failure was in.
func failureDetail(failed error, describe func(error) string) string {
	failures := loop.Failures(failed)

	details := make([]string, 0, len(failures))
	for _, failure := range failures {
		details = append(details, repositoryDetail(failure, describe))
	}

	return strings.Join(details, "; ")
}

// repositoryDetail is one failure's detail, prefixed by the repository it was
// in when it was one of several. A repository that is one no longer is said
// so plainly: the web's wording for it is about where the server runs.
func repositoryDetail(failure error, describe func(error) string) string {
	repository, named := errors.AsType[loop.RepositoryError](failure)
	if !named {
		return describe(failure)
	}

	if errors.Is(repository.Err, gitrepo.ErrNotARepository) {
		return repository.Repository + " is no longer a git repository"
	}

	return repository.Repository + ": " + describe(repository.Err)
}
