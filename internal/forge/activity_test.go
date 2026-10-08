// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// githubAna is GitHub's answer to who the token belongs to: ana.
const githubAna = `{"login":"ana"}`

// githubFoundNothing is GitHub's search finding nothing.
const githubFoundNothing = `{"total_count":0,"items":[]}`

// userPath is where either forge says who the token belongs to.
const userPath = "/user"

// pull3Reviews is where GitHub lists the reviews of o/r's pull 3, and
// reviewedPull3 is a review of it as described tells one.
const (
	pull3Reviews  = "/repos/o/r/pulls/3/reviews"
	reviewedPull3 = "reviewed o/r#3"
)

// activityForge answers each request by the first route whose key its path
// and query contain, or with an empty list.
func activityForge(t *testing.T, routes map[string]string) forge.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		asked := request.URL.EscapedPath() + "?" + request.URL.Query().Encode()
		body := "[]"

		for key, answer := range routes {
			if strings.Contains(asked, key) {
				body = answer
			}
		}

		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return forge.New(server.Client().Do, server.URL, secret)
}

func TestGitHubActivityIsWhatYouOpenedMergedAndReviewed(t *testing.T) {
	t.Parallel()

	// Arrange
	start := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	pull := func(number, at string) string {
		return `{"number":` + number + `,"title":"pull ` + number + `","html_url":"https://github.com/o/r/pull/` + number +
			`","repository_url":"https://api.github.com/repos/o/r","created_at":"` + at + `",` +
			`"pull_request":{"merged_at":"` + at + `"}}`
	}
	client := activityForge(t, map[string]string{
		"/user?":                     githubAna,
		"author%3A%40me+created":     `{"total_count":1,"items":[` + pull("1", "2026-10-02T09:00:00Z") + `]}`,
		"author%3A%40me+is%3Amerged": `{"total_count":1,"items":[` + pull("2", "2026-10-02T10:00:00Z") + `]}`,
		"reviewed-by%3A%40me":        `{"total_count":1,"items":[` + pull("3", "2026-10-01T08:00:00Z") + `]}`,
		pull3Reviews: `[{"user":{"login":"ana"},"submitted_at":"2026-10-02T11:00:00Z"},` +
			`{"user":{"login":"ben"},"submitted_at":"2026-10-02T12:00:00Z"}]`,
	})

	// Act
	activity, err := client.Activity(t.Context(), forge.KindGitHub, start, start.Add(24*time.Hour))

	// Assert
	want := []string{"opened o/r#1", "merged o/r#2", reviewedPull3}
	if got := described(activity.Events); err != nil || strings.Join(got, ", ") != strings.Join(want, ", ") {
		t.Errorf("Activity = %v, %v; want %v", got, err, want)
	}
}

func TestGitHubReviewsAreReadOldestFirstAndOnlyAsManyAsAreLookedUp(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitHub's search ranks by best match unless told otherwise, so a period
	// some way back would be read from whichever pull requests matched best;
	// the ones touched earliest are the ones most likely to be in it.
	start := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	client := activityForge(t, map[string]string{
		"/user?":         githubAna,
		"author%3A%40me": githubFoundNothing,
		"order=asc&per_page=20&q=is%3Apr+reviewed-by%3A%40me+updated%3A%3E%3D2026-10-02T00%3A00%3A00Z&sort=updated": `{` +
			`"total_count":21,"items":[{"number":3,"title":"pull 3","html_url":"https://github.com/o/r/pull/3",` +
			`"repository_url":"https://api.github.com/repos/o/r"}]}`,
		pull3Reviews: `[{"user":{"login":"ana"},"submitted_at":"2026-10-02T11:00:00Z"}]`,
	})

	// Act
	activity, err := client.Activity(t.Context(), forge.KindGitHub, start, start.Add(24*time.Hour))

	// Assert
	if got := described(activity.Events); err != nil || len(got) != 1 || got[0] != reviewedPull3 ||
		!activity.Truncated {
		t.Errorf("Activity = %v (more: %v), %v; want the one review, and that there were more", got,
			activity.Truncated, err)
	}
}

func TestGitHubActivityCountsAReviewPastTheFirstPage(t *testing.T) {
	t.Parallel()

	// Arrange
	// A hundred reviews of ben's fill the first page; ana's is the 101st.
	start := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	bens := strings.TrimSuffix(strings.Repeat(
		`{"user":{"login":"ben"},"submitted_at":"2026-10-02T10:00:00Z"},`, 100), ",")
	client, _ := recordingForge(t, func(asked recorded) (int, string) {
		query, _ := url.ParseQuery(asked.query)

		switch {
		case asked.path == userPath:
			return http.StatusOK, githubAna
		case strings.Contains(query.Get("q"), "reviewed-by:"):
			return http.StatusOK, `{"total_count":1,"items":[{"number":3,"title":"pull 3",` +
				`"html_url":"https://github.com/o/r/pull/3","repository_url":"https://api.github.com/repos/o/r"}]}`
		case asked.path == pull3Reviews && query.Get("page") == "2":
			return http.StatusOK, `[{"user":{"login":"ana"},"submitted_at":"2026-10-02T11:00:00Z"}]`
		case asked.path == pull3Reviews:
			return http.StatusOK, "[" + bens + "]"
		default:
			return http.StatusOK, githubFoundNothing
		}
	})

	// Act
	activity, err := client.Activity(t.Context(), forge.KindGitHub, start, start.Add(24*time.Hour))

	// Assert
	if got := described(activity.Events); err != nil || len(got) != 1 || got[0] != reviewedPull3 {
		t.Errorf("Activity = %v, %v; want ana's review on the second page", got, err)
	}
}

func TestGitLabActivityIsReadFromYourEvents(t *testing.T) {
	t.Parallel()

	// Arrange
	start := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	event := func(action, at string, iid string) string {
		return `{"action_name":"` + action + `","target_type":"MergeRequest","target_iid":` + iid +
			`,"target_title":"mr ` + iid + `","created_at":"` + at + `"}`
	}
	client := activityForge(t, map[string]string{
		"/events": `[` + event("opened", "2026-10-02T09:00:00Z", "4") + `,` +
			event("accepted", "2026-10-02T10:00:00Z", "5") + `,` +
			event("approved", "2026-10-02T11:00:00Z", "6") + `,` +
			event("opened", "2026-10-01T23:00:00Z", "7") + `,` +
			`{"action_name":"pushed to","target_type":null,"created_at":"2026-10-02T12:00:00Z"}]`,
	})

	// Act
	activity, err := client.Activity(t.Context(), forge.KindGitLab, start, start.Add(24*time.Hour))

	// Assert
	want := []string{"opened !4", "merged !5", "reviewed !6"}
	if got := described(activity.Events); err != nil || strings.Join(got, ", ") != strings.Join(want, ", ") {
		t.Errorf("Activity = %v, %v; want %v", got, err, want)
	}
}

func TestGitLabActivityPastThePageCapSaysThereWasMore(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitLab lists every kind of event, oldest first, so a busy period fills
	// the pages the client reads before it reaches the newest.
	start := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	fullPages := 21
	client, _ := recordingForge(t, func(asked recorded) (int, string) {
		query, _ := url.ParseQuery(asked.query)
		if page, _ := strconv.Atoi(query.Get("page")); page > fullPages {
			return http.StatusOK, "[]"
		}

		return http.StatusOK, "[" + strings.TrimSuffix(strings.Repeat(
			`{"action_name":"opened","target_type":"MergeRequest","target_iid":1,`+
				`"created_at":"2026-10-02T09:00:00Z"},`, 100), ",") + "]"
	})

	// Act
	activity, err := client.Activity(t.Context(), forge.KindGitLab, start, start.Add(24*time.Hour))

	// Assert
	if err != nil || len(activity.Events) == 0 || !activity.Truncated {
		t.Errorf("Activity = %d events (more: %v), %v; want those read, and that there were more",
			len(activity.Events), activity.Truncated, err)
	}
}

func TestGitHubActivityPastWhatTheSearchServesSaysThereWasMore(t *testing.T) {
	t.Parallel()

	// GitHub's search serves its first 1,000 results however many it finds.
	pull := `{"number":1,"title":"pull 1","html_url":"https://github.com/o/r/pull/1",` +
		`"repository_url":"https://api.github.com/repos/o/r","created_at":"2026-10-02T09:00:00Z",` +
		`"pull_request":{"merged_at":"2026-10-02T09:00:00Z"}}`
	cases := map[string]string{"the opened search": "created:", "the merged search": "merged:"}

	for name, search := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			start := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
			client, _ := recordingForge(t, func(asked recorded) (int, string) {
				query, _ := url.ParseQuery(asked.query)

				switch {
				case asked.path == userPath:
					return http.StatusOK, githubAna
				case strings.Contains(query.Get("q"), search):
					return http.StatusOK, `{"total_count":1500,"items":[` +
						strings.TrimSuffix(strings.Repeat(pull+",", 100), ",") + `]}`
				default:
					return http.StatusOK, githubFoundNothing
				}
			})

			// Act
			activity, err := client.Activity(t.Context(), forge.KindGitHub, start, start.Add(24*time.Hour))

			// Assert
			if err != nil || len(activity.Events) != 1000 || !activity.Truncated {
				t.Errorf("Activity = %d events (more: %v), %v; want the 1,000 served, and that there were more",
					len(activity.Events), activity.Truncated, err)
			}
		})
	}
}

// described is each event as "verb repository#number", or "verb !number" on
// GitLab, which names no repository.
func described(events []forge.Event) []string {
	verbs := map[forge.EventKind]string{
		forge.EventOpened: "opened", forge.EventMerged: "merged", forge.EventReviewed: "reviewed",
	}

	lines := make([]string, 0, len(events))
	for _, event := range events {
		if event.Repository == "" {
			lines = append(lines, verbs[event.Kind]+" !"+strconv.Itoa(event.Number))

			continue
		}

		lines = append(lines, verbs[event.Kind]+" "+event.Repository+"#"+strconv.Itoa(event.Number))
	}

	return lines
}
