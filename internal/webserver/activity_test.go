// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// activityPath is where the Summary is read.
const activityPath = "/api/activity"

// wednesday is when the server's clock says it is, so the period it opens on
// is Tuesday.
func wednesday() time.Time { return time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC) }

// activityDeps answer as a day's work: a commit at nine on Tuesday, and the
// forge read failing with its host in its words; each read's start is kept.
func activityDeps(starts *[]time.Time) webserver.Deps {
	deps := filledDeps()
	deps.Clock = wednesday
	deps.CommitsBetween = func(start, _ time.Time) []loop.RepositoryCommits {
		*starts = append(*starts, start)

		return []loop.RepositoryCommits{{Repository: "", Failed: nil, Commits: []gitrepo.DatedCommit{{
			Hash: "abc1234ffff", Short: "abc1234", Subject: "Fix the leak",
			Authored: time.Date(2026, 9, 15, 9, 30, 0, 0, time.UTC),
		}}}}
	}
	deps.ForgeActivity = func(time.Time, time.Time) (forge.Activity, error) {
		return forge.Activity{}, fmt.Errorf("%w: https://git.internal.example", forge.ErrUnreachable)
	}
	deps.JiraActivity = nil

	return deps
}

func TestTheSummaryIsThePreviousWorkingDayByDefault(t *testing.T) {
	t.Parallel()

	// Arrange
	var starts []time.Time

	handler := serve(t, activityDeps(&starts), config.Default())

	// Act
	recorder := send(t, handler, http.MethodGet, activityPath, "")

	// Assert
	got := decode[api.Activity](t, recorder)
	if recorder.Code != http.StatusOK || got.From.String() != "2026-09-15" || got.To.String() != "2026-09-15" ||
		got.Today.String() != "2026-09-16" {
		t.Errorf("status %d, period %s to %s, today %s; want Tuesday, today Wednesday",
			recorder.Code, got.From, got.To, got.Today)
	}

	if len(starts) != 1 || !starts[0].Equal(time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("git read from %v, want Tuesday's midnight", starts)
	}
}

func TestTheSummaryNestsItsItemsAndWritesThemAsMarkdown(t *testing.T) {
	t.Parallel()

	// Arrange
	var starts []time.Time

	handler := serve(t, activityDeps(&starts), config.Default())

	// Act
	got := decode[api.Activity](t, send(t, handler, http.MethodGet, activityPath, ""))

	// Assert
	month := got.Years[0].Months[0]
	day := month.Days[0]
	item := day.Hours[0].Items[0]

	if month.Name != "September" || day.Weekday != "Tuesday" || day.Hours[0].Label != "09:00" ||
		item.Verb != "committed" || item.Ref != "abc1234" || item.Source != api.ActivitySourceNameGit {
		t.Errorf("years = %+v, want the commit under Tuesday at 09:00", got.Years)
	}

	if !strings.Contains(got.Text, "- committed abc1234 Fix the leak") {
		t.Errorf("text = %q, want the commit as Markdown", got.Text)
	}
}

func TestASourceThatCannotBeReadIsNamedWithoutItsHost(t *testing.T) {
	t.Parallel()

	// Arrange
	var starts []time.Time

	handler := serve(t, activityDeps(&starts), config.Default())

	// Act
	recorder := send(t, handler, http.MethodGet, activityPath+"?from=2026-09-14&to=2026-09-15", "")

	// Assert
	got := decode[api.Activity](t, recorder)
	if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), "git.internal.example") {
		t.Fatalf("status %d, body %s; want 200 and no host", recorder.Code, recorder.Body.String())
	}

	var forgeSource api.ActivitySource

	for _, source := range got.Sources {
		if source.Source == api.ActivitySourceNameForge {
			forgeSource = source
		}
	}

	if forgeSource.State != api.ActivitySourceStateFailed || forgeSource.Detail == "" || forgeSource.Name != "The forge" {
		t.Errorf("sources = %+v, want the forge named as failed, with why", got.Sources)
	}
}

func TestAPeriodThatCannotBeSummedUpIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"backwards": "?from=2026-09-15&to=2026-09-14",
		"too long":  "?from=2025-01-01&to=2026-09-15",
	}

	for name, query := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var starts []time.Time

			handler := serve(t, activityDeps(&starts), config.Default())

			// Act
			recorder := send(t, handler, http.MethodGet, activityPath+query, "")

			// Assert
			if recorder.Code != http.StatusUnprocessableEntity || len(starts) != 0 {
				t.Errorf("status %d, read %v; want 422 and nothing read", recorder.Code, starts)
			}
		})
	}
}

func TestOneDayGivenIsThatDayAlone(t *testing.T) {
	t.Parallel()

	for _, query := range []string{"?from=2026-09-10", "?to=2026-09-10"} {
		t.Run(query, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var starts []time.Time

			handler := serve(t, activityDeps(&starts), config.Default())

			// Act
			recorder := send(t, handler, http.MethodGet, activityPath+query, "")

			// Assert
			got := decode[api.Activity](t, recorder)
			if got.From.String() != "2026-09-10" || got.To.String() != "2026-09-10" {
				t.Errorf("period %s to %s, want 2026-09-10 alone", got.From, got.To)
			}
		})
	}
}

func TestEverySourceTheServerReachesIsRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// With no BrowseURL wired an issue still reads, unlinked.
	var starts []time.Time

	deps := activityDeps(&starts)
	deps.Tasks.Touched = func(time.Time) ([]taskwarrior.Task, error) { return nil, nil }
	deps.JiraActivity = func(start, _ time.Time) (jira.Activity, error) {
		return jira.Activity{Events: []jira.Event{{At: start.Add(time.Hour), Kind: jira.EventMoved, Key: "PROJ-1"}}}, nil
	}
	deps.BrowseURL = nil

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodGet, activityPath, "")

	// Assert
	got := decode[api.Activity](t, recorder)

	named := make([]api.ActivitySourceName, 0, len(got.Sources))
	for _, source := range got.Sources {
		named = append(named, source.Source)
	}

	want := []api.ActivitySourceName{
		api.ActivitySourceNameGit, api.ActivitySourceNameTasks, api.ActivitySourceNameJira, api.ActivitySourceNameForge,
	}
	if !slices.Equal(named, want) || !strings.Contains(got.Text, "PROJ-1") {
		t.Errorf("sources %v, text %q; want %v and the issue", named, got.Text, want)
	}
}

// sourceDetail is the detail the Summary gives for source, read with deps.
func sourceDetail(t *testing.T, deps webserver.Deps, source api.ActivitySourceName) string {
	t.Helper()

	recorder := send(t, serve(t, deps, config.Default()), http.MethodGet, activityPath, "")
	for _, read := range decode[api.Activity](t, recorder).Sources {
		if read.Source == source {
			return read.Detail
		}
	}

	t.Fatalf("no %s source in %s", source, recorder.Body.String())

	return ""
}

func TestASourceThatCannotBeReadSaysWhatToDo(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		source api.ActivitySourceName
		fail   func(*webserver.Deps)
		want   string
	}{
		"git with no user.email": {
			source: api.ActivitySourceNameGit,
			fail: func(deps *webserver.Deps) {
				deps.CommitsBetween = func(time.Time, time.Time) []loop.RepositoryCommits {
					return []loop.RepositoryCommits{{Repository: "", Commits: nil, Failed: gitrepo.ErrNoIdentity}}
				}
			},
			want: "git config user.email",
		},
		"a remote naming no forge repository": {
			source: api.ActivitySourceNameForge,
			fail: func(deps *webserver.Deps) {
				deps.ForgeActivity = func(time.Time, time.Time) (forge.Activity, error) {
					return forge.Activity{}, fmt.Errorf("%w: https://git.internal.example/", forge.ErrNotARemote)
				}
			},
			want: "origin",
		},
		"a forge that cannot be told": {
			source: api.ActivitySourceNameForge,
			fail: func(deps *webserver.Deps) {
				deps.ForgeActivity = func(time.Time, time.Time) (forge.Activity, error) {
					return forge.Activity{}, fmt.Errorf("%w: git.internal.example", forge.ErrUnknownForge)
				}
			},
			want: "forge.kind",
		},
	}

	for name, failing := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var starts []time.Time

			deps := activityDeps(&starts)
			failing.fail(&deps)

			// Act
			detail := sourceDetail(t, deps, failing.source)

			// Assert
			if !strings.Contains(detail, failing.want) || strings.Contains(detail, "internal.example") {
				t.Errorf("detail = %q, want it to say %q and never the host", detail, failing.want)
			}
		})
	}
}

func TestASourceNotSetUpIsMarkedApartFromOneThatFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	var starts []time.Time

	deps := activityDeps(&starts)
	deps.CommitsBetween = func(time.Time, time.Time) []loop.RepositoryCommits {
		return []loop.RepositoryCommits{{Repository: "", Commits: nil, Failed: gitrepo.ErrNoIdentity}}
	}
	deps.JiraActivity = func(time.Time, time.Time) (jira.Activity, error) {
		return jira.Activity{}, fmt.Errorf("%w: reading the token", jira.ErrNoCredential)
	}
	deps.Tasks.Touched = func(time.Time) ([]taskwarrior.Task, error) { return nil, taskwarrior.ErrNotInstalled }

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodGet, activityPath, "")

	// Assert
	states := map[api.ActivitySourceName]api.ActivitySourceState{}
	for _, source := range decode[api.Activity](t, recorder).Sources {
		states[source.Source] = source.State
	}

	want := map[api.ActivitySourceName]api.ActivitySourceState{
		api.ActivitySourceNameGit:   api.ActivitySourceStateNotSetUp,
		api.ActivitySourceNameTasks: api.ActivitySourceStateNotSetUp,
		api.ActivitySourceNameJira:  api.ActivitySourceStateNotSetUp,
		api.ActivitySourceNameForge: api.ActivitySourceStateFailed,
	}
	for source, state := range want {
		if states[source] != state {
			t.Errorf("%s is %q, want %q", source, states[source], state)
		}
	}
}

func TestARepositoryThatCannotBeReadIsNamedInWhatToDo(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		failed error
		want   []string
		never  string
	}{
		"a favorite with no user.email": {
			failed: gitrepo.ErrNoIdentity,
			want:   []string{"acme/web: ", "git config user.email"},
			never:  "",
		},
		"a favorite that is a repository no longer": {
			// The server runs in a repository; only the favorite is not one.
			failed: fmt.Errorf("%w: /home/me/src/web", gitrepo.ErrNotARepository),
			want:   []string{"acme/web is no longer a git repository"},
			never:  "the server",
		},
	}

	for name, failing := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var starts []time.Time

			deps := activityDeps(&starts)
			read := deps.CommitsBetween
			deps.CommitsBetween = func(start, end time.Time) []loop.RepositoryCommits {
				here := read(start, end)
				here[0].Repository = "acme/api"

				return append(here, loop.RepositoryCommits{Repository: "acme/web", Commits: nil, Failed: failing.failed})
			}

			// Act
			detail := sourceDetail(t, deps, api.ActivitySourceNameGit)

			// Assert
			for _, want := range failing.want {
				if !strings.Contains(detail, want) {
					t.Errorf("detail = %q, want it to say %q", detail, want)
				}
			}

			if failing.never != "" && strings.Contains(detail, failing.never) || strings.Contains(detail, "/home/me") {
				t.Errorf("detail = %q, want neither %q nor the favorite's path", detail, failing.never)
			}
		})
	}
}

func TestTheSummarySaysHowLongItIsForTheServiceItPostsTo(t *testing.T) {
	t.Parallel()

	// Arrange
	var starts []time.Time

	cfg := config.Default()
	cfg.Messaging = config.Messaging{Kind: config.KindDiscord, WebhookURL: "https://discord.example/api/webhooks/1/x"}
	handler := serve(t, activityDeps(&starts), cfg)

	// Act
	got := decode[api.Activity](t, send(t, handler, http.MethodGet, activityPath, ""))

	// Assert
	want := loop.SummaryLength(config.KindDiscord, got.Text)
	if got.PostLength == nil || got.PostLength.Service != "Discord" || got.PostLength.Count != want.Count ||
		got.PostLength.Limit != 2000 || got.PostLength.Unit != api.PostLengthUnitCharacters {
		t.Errorf("post_length = %+v, want the text measured for Discord: %+v", got.PostLength, want)
	}
}

func TestTheSummarySaysNoLengthWithNowhereToPost(t *testing.T) {
	t.Parallel()

	// Arrange
	var starts []time.Time

	handler := serve(t, activityDeps(&starts), config.Default())

	// Act
	got := decode[api.Activity](t, send(t, handler, http.MethodGet, activityPath, ""))

	// Assert
	if got.PostLength != nil {
		t.Errorf("post_length = %+v, want none with no messaging set up", got.PostLength)
	}
}
