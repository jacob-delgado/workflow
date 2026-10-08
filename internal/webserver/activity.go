// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// GetActivity reads back what you did over the period asked, or the previous
// working day, in the server's own time zone: each source on its own, so one
// that cannot be read is named and the rest still answer.
func (s *server) GetActivity(
	_ context.Context, request api.GetActivityRequestObject,
) (api.GetActivityResponseObject, error) {
	now := s.now()
	today := activity.DateOf(now)

	period, err := activity.PeriodAsked(dayAsked(request.Params.From), dayAsked(request.Params.To), today)
	if err != nil {
		return periodRefused(err), nil
	}

	start, end := period.Bounds(now.Location())
	summary := activity.Summary{Period: period, Reads: s.activityReads(start, end)}

	report := ActivityReport(summary, today, now.Location(), s.describe)
	report.PostLength = PostLength(s.config().Messaging, report.Text)

	return api.GetActivity200JSONResponse(report), nil
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

// periodRefused is a period that could not be read as asked, and why.
func periodRefused(err error) api.GetActivityResponseObject {
	return api.GetActivity422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable, periodReason(err)))
}

// periodReason says why a period of days that are each a date cannot be
// summed up: it runs backwards, or for too long. A day that is no date never
// gets here; the contract refuses it.
func periodReason(err error) string {
	return "the period cannot be summed up: " + err.Error()
}

// dayAsked is a day the contract read as a date, written year-month-day, or
// empty when none was asked.
func dayAsked(day *openapi_types.Date) string {
	if day == nil {
		return ""
	}

	return day.String()
}

// wireDate is a day as the contract writes a date.
func wireDate(day activity.Date) openapi_types.Date {
	return openapi_types.Date{Time: time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)}
}

// Trade-off TRADE-32: a posted Summary is not recorded, as an announcement is.

// PostActivity posts the Summary's Markdown for the period asked, rendered for
// the service, to the channel asked or the configured one, or a webhook's own.
// A period that cannot be read and a blank text are refused before anything
// is posted; a post that fails is classified by fault, whose details never
// carry the error's own text, which can name the webhook.
func (s *server) PostActivity(
	_ context.Context, request api.PostActivityRequestObject,
) (api.PostActivityResponseObject, error) {
	if s.deps.Post == nil {
		return activityPostRefused("posting is not available"), nil
	}

	body := *request.Body

	period, err := activity.PeriodAsked(body.From.String(), body.To.String(), activity.DateOf(s.now()))
	if err != nil {
		return activityPostRefused(periodReason(err)), nil
	}

	settings, channel := s.config().Messaging, s.channelOr(orZero(body.Channel))

	err = loop.PostSummary(s.deps.Post, settings.Kind, channel, body.Text)
	if errors.Is(err, loop.ErrEmptySummary) {
		return activityPostRefused(err.Error()), nil
	}

	if errors.Is(err, messaging.ErrTooLong) {
		return api.PostActivity422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeTooLong, err.Error())), nil
	}

	if err != nil {
		return problemAnswer[api.PostActivitydefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
	}

	return api.PostActivity200JSONResponse{
		From: wireDate(period.From), To: wireDate(period.To), Channel: channel,
		Destination: destination(channel, settings), Text: body.Text,
	}, nil
}

// activityPostRefused is the 422 refusing a Summary the server will not post.
func activityPostRefused(detail string) api.PostActivity422ApplicationProblemPlusJSONResponse {
	return api.PostActivity422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable, detail))
}

// destination is where a post to channel goes, in words: the channel, or
// where the configuration sends a post that names none — a webhook's own.
func destination(channel string, settings config.Messaging) string {
	if channel != "" {
		return channel
	}

	return settings.Target()
}

// activityReads asks every source the server reaches, one after another.
func (s *server) activityReads(start, end time.Time) []activity.Read {
	return loop.ReadAll(loop.SummaryReads(loop.ActivitySeams{
		Commits: s.deps.CommitsBetween, Touched: s.deps.Tasks.Touched,
		Jira: s.deps.JiraActivity, BrowseURL: s.deps.BrowseURL,
		Forge: s.deps.ForgeActivity, ForgeKind: s.forgeKindNow(),
	}, start, end))
}

// ActivityReport is the summary as the API answers it, today being the day it
// is in loc, so the command line prints the same shape the web reads. Each
// failure is worded by describe, which the server and the command line both
// make FaultDetail, so it never names a host.
func ActivityReport(
	summary activity.Summary, today activity.Date, loc *time.Location, describe func(error) string,
) api.Activity {
	sources := make([]api.ActivitySource, 0, len(summary.Reads))

	for _, read := range summary.Reads {
		sources = append(sources, sourceDTO(read, describe))
	}

	return api.Activity{
		From: wireDate(summary.Period.From), To: wireDate(summary.Period.To), Today: wireDate(today),
		Sources: sources, Years: yearsDTO(activity.Group(summary.Items(), loc)), Text: summary.Text(loc),
	}
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
// so plainly: fault's wording for it is about where the server runs.
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

// describe is a failure's detail as fault words it, handing one it does not
// recognize to Unexpected.
func (s *server) describe(err error) string {
	body := s.fault(err)

	return body.Detail
}

// FaultDetail is a failure's detail as the web's problem words it: the same
// sentence for the same class of failure, never naming a host or a path.
func FaultDetail(err error) string {
	prob, _ := faultProblem(err)

	return prob.Detail
}
