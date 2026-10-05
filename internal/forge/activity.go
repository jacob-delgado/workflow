// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"cmp"
	"context"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// reviewLookups bounds how many reviewed pull requests GitHub's activity reads
// the reviews of, one request each, to place a review in time.
const reviewLookups = 20

// sortParameter is the query parameter GitHub and GitLab both order a list by.
const sortParameter = "sort"

// EventKind is what you did to a pull or merge request.
type EventKind int

const (
	// EventOpened is one you opened.
	EventOpened EventKind = iota + 1
	// EventMerged is one of yours merged, on GitHub, or one you merged, on GitLab.
	EventMerged
	// EventReviewed is one you reviewed, on GitHub, or approved, on GitLab.
	EventReviewed
)

// Event is one thing you did to a pull or merge request, and when.
// Repository is its owner/name on GitHub, and empty on GitLab, whose events
// name a project only by its id.
type Event struct {
	At         time.Time
	Kind       EventKind
	Number     int
	Title      string
	URL        string
	Repository string
}

// Activity is what you did on the forge over a period, oldest first, and
// whether there was more than was read.
type Activity struct {
	Events    []Event
	Truncated bool
}

// Activity reads what you did to pull or merge requests on the forge, in any
// repository, from start up to end.
func (c Client) Activity(ctx context.Context, kind Kind, start, end time.Time) (Activity, error) {
	speaks, err := dialectFor(kind)
	if err != nil {
		return Activity{}, err
	}

	activity, err := speaks.activity(ctx, c, start, end)
	if err != nil {
		return Activity{}, err
	}

	activity.Events = slices.DeleteFunc(activity.Events, func(event Event) bool {
		return event.At.Before(start) || !event.At.Before(end)
	})
	slices.SortStableFunc(activity.Events, func(a, b Event) int {
		return cmp.Or(a.At.Compare(b.At), cmp.Compare(a.Kind, b.Kind))
	})

	return activity, nil
}

// githubActivityItem is a pull request as the search answers it.
type githubActivityItem struct {
	githubReviewItem

	PullRequest struct {
		MergedAt time.Time `json:"merged_at"`
	} `json:"pull_request"`
}

func (g githubActivityItem) event(kind EventKind, at time.Time) Event {
	_, repository, _ := strings.Cut(g.RepositoryURL, "/repos/")

	return Event{At: at, Kind: kind, Number: g.Number, Title: g.Title, URL: g.URL, Repository: repository}
}

// githubActivity searches for the pull requests you opened, had merged and
// reviewed over the period. A review's time is not in the search, so the
// reviews of each reviewed pull request are read, up to reviewLookups.
func githubActivity(ctx context.Context, client Client, start, end time.Time) (Activity, error) {
	span := start.UTC().Format(time.RFC3339) + ".." + end.UTC().Format(time.RFC3339)

	opened, err := githubSearch[githubActivityItem](ctx, client, "is:pr author:@me created:"+span)
	if err != nil {
		return Activity{}, err
	}

	merged, err := githubSearch[githubActivityItem](ctx, client, "is:pr author:@me is:merged merged:"+span)
	if err != nil {
		return Activity{}, err
	}

	var activity Activity

	for _, item := range opened {
		activity.Events = append(activity.Events, item.event(EventOpened, item.CreatedAt))
	}

	for _, item := range merged {
		activity.Events = append(activity.Events, item.event(EventMerged, item.PullRequest.MergedAt))
	}

	reviewed, truncated, err := githubReviewed(ctx, client, start)
	activity.Events, activity.Truncated = append(activity.Events, reviewed...), truncated

	return activity, err
}

// githubReviewed is each review you gave a pull request updated since start,
// and whether there were more such pull requests than reviewLookups.
func githubReviewed(ctx context.Context, client Client, start time.Time) ([]Event, bool, error) {
	reviewer, err := client.Whoami(ctx)
	if err != nil {
		return nil, false, err
	}

	// One page, the earliest touched first: GitHub ranks a search by best
	// match otherwise, and only reviewLookups are looked up anyway.
	query := url.Values{
		"q":           {"is:pr reviewed-by:@me updated:>=" + start.UTC().Format(time.RFC3339)},
		sortParameter: {"updated"},
		"order":       {"asc"}, "per_page": {strconv.Itoa(reviewLookups)},
	}

	found, err := call[githubSearchPage[githubActivityItem]](ctx, client, http.MethodGet,
		"/search/issues?"+query.Encode(), nil)
	if err != nil {
		return nil, false, err
	}

	var events []Event

	for _, item := range found.Items {
		_, repository, _ := strings.Cut(item.RepositoryURL, "/repos/")

		reviews, err := call[[]githubReview](ctx, client, http.MethodGet,
			"/repos/"+escapedPath(repository)+"/pulls/"+strconv.Itoa(item.Number)+"/reviews?per_page=100", nil)
		if err != nil {
			return nil, false, err
		}

		for _, review := range reviews {
			if review.User.Login == reviewer.Name() {
				events = append(events, item.event(EventReviewed, review.SubmittedAt))
			}
		}
	}

	return events, found.TotalCount > len(found.Items), nil
}

// gitlabEvent is one of your events as GitLab lists it.
type gitlabEvent struct {
	Action      string    `json:"action_name"`
	TargetType  string    `json:"target_type"`
	TargetIID   int       `json:"target_iid"`
	TargetTitle string    `json:"target_title"`
	CreatedAt   time.Time `json:"created_at"`
}

// gitlabActivity reads your events over the period and keeps those on a merge
// request: opened, merged (accepted, in GitLab's words) and approved. GitLab
// filters events by day, exclusive of both, so the days around the period
// are asked for and each event is kept by its own time.
func gitlabActivity(ctx context.Context, client Client, start, end time.Time) (Activity, error) {
	query := url.Values{
		"after":       {start.UTC().AddDate(0, 0, -1).Format(time.DateOnly)},
		"before":      {end.UTC().AddDate(0, 0, 1).Format(time.DateOnly)},
		sortParameter: {"asc"},
	}

	listed, err := readPages(func(page int) ([]gitlabEvent, int, error) {
		one, err := call[[]gitlabEvent](ctx, client, http.MethodGet, "/events?"+pageQuery(query, page), nil)

		return one, uncounted, err
	})
	if err != nil {
		return Activity{}, err
	}

	kinds := map[string]EventKind{"opened": EventOpened, "accepted": EventMerged, "approved": EventReviewed}

	var activity Activity

	for _, event := range listed {
		if kind, ok := kinds[event.Action]; ok && event.TargetType == "MergeRequest" {
			activity.Events = append(activity.Events,
				Event{At: event.CreatedAt, Kind: kind, Number: event.TargetIID, Title: event.TargetTitle})
		}
	}

	return activity, nil
}
