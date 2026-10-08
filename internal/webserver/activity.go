// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/report"
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

	answer := report.Activity(summary, today, now.Location(), s.describe)
	answer.PostLength = report.PostLength(s.config().Messaging, answer.Text)

	return api.GetActivity200JSONResponse(answer), nil
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

// Trade-off TRADE-32: a posted Summary is not recorded, as an announcement is.

// PostActivity posts the Summary's Markdown for the period asked, rendered for
// the service, to the channel asked or the configured one, or a webhook's own.
// A period that cannot be read and a blank text are refused before anything
// is posted; a post that fails is classified by fault, whose details never
// carry the error's own text, which can name the webhook.
func (s *server) PostActivity(
	_ context.Context, request api.PostActivityRequestObject,
) (api.PostActivityResponseObject, error) {
	body := *request.Body

	period, err := activity.PeriodAsked(body.From.String(), body.To.String(), activity.DateOf(s.now()))
	if err != nil {
		return activityPostRefused(periodReason(err)), nil
	}

	settings, channel := s.config().Messaging, s.channelOr(orZero(body.Channel))

	err = loop.PostSummary(s.deps.Messaging.Post, settings.Kind, channel, body.Text)
	if errors.Is(err, loop.ErrSummaryUnavailable) || errors.Is(err, loop.ErrEmptySummary) {
		return activityPostRefused(err.Error()), nil
	}

	if errors.Is(err, messaging.ErrTooLong) {
		return api.PostActivity422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeTooLong, err.Error())), nil
	}

	if err != nil {
		return problemAnswer[api.PostActivitydefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
	}

	return api.PostActivity200JSONResponse{
		From: report.Date(period.From), To: report.Date(period.To), Channel: channel,
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
		Commits: s.deps.Git.CommitsBetween, Touched: s.deps.Tasks.Touched,
		Jira: s.deps.Jira.Activity, BrowseURL: s.deps.Jira.BrowseURL,
		Forge: s.deps.Forge.Activity, ForgeKind: s.forgeKindNow(),
	}, start, end))
}

// describe is a failure's detail as fault words it, handing one it does not
// recognize to Unexpected.
func (s *server) describe(err error) string {
	body := s.fault(err)

	return body.Detail
}
