// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"net/http"
	"slices"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/demo"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// webClock is how long a web test's demo has run, moved on by the test, so
// CI passes when the test says so rather than as the wall clock runs.
type webClock struct {
	mu      sync.Mutex
	elapsed time.Duration
}

func (c *webClock) since() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.elapsed
}

func (c *webClock) pass(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.elapsed += d
}

// webDemo is the demo's world served over the web API, on a clock the test
// moves.
type webDemo struct {
	t       *testing.T
	world   *demo.World
	clock   *webClock
	handler http.Handler
}

func servedWeb(t *testing.T, options ...demo.Option) webDemo {
	t.Helper()

	clock := &webClock{}
	world := demo.New(clock.since, options...)
	handler := webtest.ServeWorld(t, world.Web(webtest.TestVersion, t.TempDir()), fstest.MapFS{})
	t.Cleanup(func() { webtest.Send(t, handler, http.MethodDelete, "/api/announce/queued", "") })

	return webDemo{t: t, world: world, clock: clock, handler: handler}
}

// frame is the next snapshot the page's event stream would push.
func (d webDemo) frame() api.Snapshot {
	d.t.Helper()

	return webtest.FirstSnapshot(d.t, webtest.StreamOnce(d.t, d.handler, "/api/events").Body.String())
}

// send makes a request as the page does and fails the test on any answer
// but OK.
func (d webDemo) send(method, target, body string) {
	d.t.Helper()

	recorder := webtest.Send(d.t, d.handler, method, target, body)
	if recorder.Code != http.StatusOK {
		d.t.Fatalf("%s %s answered %d: %s", method, target, recorder.Code, recorder.Body.String())
	}
}

// read is a GET the page makes, decoded.
func read[T any](served webDemo, target string) T {
	served.t.Helper()

	recorder := webtest.Get(served.t, served.handler, target)
	if recorder.Code != http.StatusOK {
		served.t.Fatalf("GET %s answered %d: %s", target, recorder.Code, recorder.Body.String())
	}

	return webtest.Decode[T](served.t, recorder)
}

// The page's requests for the loop, as its forms send them.
const (
	startWork  = `{"issue_key":"PROJ-412","fetch":true}`
	stageAll   = `{"all":true}`
	commitWork = `{"type":"fix","scope":"config","subject":"redact tokens in the request log","body":"","breaking":false}`
	openPull   = `{"title":"fix(config): redact tokens in the request log","base":"main",` +
		`"body":"## What\n","draft":false,"reviewers":[],"assignees":[],"labels":[]}`
)

// startProgress is the page's request for the move the demo's Jira offers
// PROJ-412 from To Do.
const startProgress = `{"transition_id":"11","fields":[]}`

// workedBranch is the branch starting work on PROJ-412 makes.
const workedBranch = "fix/PROJ-412-redact-tokens-in-the-request-log"

// The code owners of the redaction's files.
const (
	sam = "sam.ortiz"
	lee = "lee.chen"
)

// committed starts work on PROJ-412, moves it into progress, and commits all
// of it.
func (d webDemo) committed() {
	d.t.Helper()

	d.send(http.MethodPost, "/api/branches", startWork)
	d.send(http.MethodPost, "/api/issues/PROJ-412/transitions", startProgress)
	d.send(http.MethodPost, "/api/stage", stageAll)
	d.send(http.MethodPost, "/api/commit", commitWork)
}

// pushed is committed, then pushed.
func (d webDemo) pushed() {
	d.t.Helper()

	d.committed()
	d.send(http.MethodPost, "/api/push", "")
}

// opened is pushed, with #128 opened, and what the open answered.
func (d webDemo) opened() api.OpenedPullRequest {
	d.t.Helper()

	d.pushed()

	recorder := webtest.Send(d.t, d.handler, http.MethodPost, "/api/pull-request", openPull)
	if recorder.Code != http.StatusOK {
		d.t.Fatalf("opening the pull request answered %d: %s", recorder.Code, recorder.Body.String())
	}

	return webtest.Decode[api.OpenedPullRequest](d.t, recorder)
}

// ciPassesOnTheWeb is long enough on the demo's clock for every check to pass
// and for the server's cached CI, read at most every two seconds, to go stale.
const ciPassesOnTheWeb = 5 * time.Second

// forgeAskedAgainOnTheWeb is long enough on the demo's clock for the pull
// request an open put in the server's cache to go stale, so the forge is asked
// again, and short of the slowest check passing.
const forgeAskedAgainOnTheWeb = 2500 * time.Millisecond

// branchOf is where a frame's branch stands: its name, its head, its
// upstream and how many changes are left in the work tree.
type branchOf struct {
	name, head, upstream string
	changes              int
}

func standing(frame api.Snapshot) branchOf {
	return branchOf{frame.Branch.Name, frame.Branch.Head, frame.Branch.Upstream, len(frame.Changes.Changes)}
}

// ciOf is how a frame's CI reads, or none when it shows no pull request's.
func ciOf(frame api.Snapshot) api.CIState {
	if frame.Review.Ci == nil {
		return api.CIStateNone
	}

	return frame.Review.Ci.State
}

// heldOf is the state of a frame's announcement waiting for CI, or none.
func heldOf(frame api.Snapshot) api.QueuedAnnouncementState {
	if frame.QueuedAnnouncement == nil {
		return ""
	}

	return frame.QueuedAnnouncement.State
}

// reviewOf is how a frame's review reads: CI, whether the pull request was
// announced, and the state of an announcement waiting for CI.
type reviewOf struct {
	ci        api.CIState
	announced bool
	held      api.QueuedAnnouncementState
}

func reviewing(frame api.Snapshot) reviewOf {
	return reviewOf{ciOf(frame), frame.Review.Announced, heldOf(frame)}
}

// announcedPulls are the pull requests the store holds announced.
func announcedPulls(d webDemo) []int {
	announced := d.world.Deps().Store.Announced(d.t.Context())
	pulls := make([]int, 0, len(announced))

	for _, announced := range announced {
		pulls = append(pulls, announced.Pull)
	}

	return pulls
}

func TestTheWebLoopRunsFromAnIssueToAnAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

	// Act: open the page
	first := demo.frame()

	// Assert: PROJ-412 leads the list, on main
	if got := [2]string{first.Branch.Name, first.Issues.Issues[0].Key}; got != [2]string{"main", string(worked)} {
		t.Fatalf("the page opens on %s with %s first, want main with %s", got[0], got[1], worked)
	}

	// Act: start work, stage everything and commit it
	demo.committed()

	// Assert: the commit is on the issue's branch, and nothing is left to stage
	if got, want := standing(demo.frame()), (branchOf{workedBranch, "4f2c9e1", "", 0}); got != want {
		t.Fatalf("after the commit the branch stands at %+v, want %+v", got, want)
	}

	// Act: push it
	demo.send(http.MethodPost, "/api/push", "")

	// Assert: origin has it
	if got := standing(demo.frame()).upstream; got != "origin/"+workedBranch {
		t.Fatalf("after the push the upstream is %q, want origin/%s", got, workedBranch)
	}

	// Act: open its pull request
	opened := webtest.Decode[api.OpenedPullRequest](t,
		webtest.Send(t, demo.handler, http.MethodPost, "/api/pull-request", openPull))

	// Assert: #128 is open, and its announcement waits for CI, which the open
	// does not read
	if opened.Pull.Number != 128 {
		t.Fatalf("the open answered #%d, want #128", opened.Pull.Number)
	}

	waiting := reviewOf{api.CIStateNone, false, api.QueuedAnnouncementStateWaiting}
	if got := reviewing(demo.frame()); got != waiting {
		t.Fatalf("after the open the review reads %+v, want %+v", got, waiting)
	}

	// Act: let the forge be asked again
	demo.clock.pass(forgeAskedAgainOnTheWeb)

	// Assert: CI is running, and the announcement still waits for it
	running := reviewOf{api.CIStateRunning, false, api.QueuedAnnouncementStateWaiting}
	if got := reviewing(demo.frame()); got != running {
		t.Fatalf("once the forge is asked again the review reads %+v, want %+v", got, running)
	}

	// Act: let CI finish
	demo.clock.pass(ciPassesOnTheWeb - forgeAskedAgainOnTheWeb)

	// Assert: CI passed, and the pull request was announced on that frame
	posted := reviewOf{api.CIStatePassed, true, api.QueuedAnnouncementStateAnnounced}
	if got := reviewing(demo.frame()); got != posted {
		t.Fatalf("once CI has had time to pass the review reads %+v, want %+v", got, posted)
	}

	if got := announcedPulls(demo); !slices.Equal(got, []int{128}) {
		t.Errorf("the store holds %v announced, want #128", got)
	}
}

func TestTheWebMovesTheWorkedIssueIntoProgressOnceStarted(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)
	demo.send(http.MethodPost, "/api/branches", startWork)

	// Act
	demo.send(http.MethodPost, "/api/issues/PROJ-412/transitions", startProgress)

	// Assert
	issues := demo.frame().Issues.Issues
	at := slices.IndexFunc(issues, func(issue api.Issue) bool { return issue.Key == string(worked) })

	if at < 0 || issues[at].Status != inProgress {
		t.Errorf("the page lists %+v, want PROJ-412 In Progress", issues)
	}
}

func TestTheWebReportsTheDemoVersionOnGitHub(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

	// Act
	health := read[api.Health](demo, "/api/health")

	// Assert
	if health.Version != webtest.TestVersion || health.ForgeKind != api.HealthForgeKindGithub || health.DryRun {
		t.Errorf("health reads %+v, want version %s on GitHub, not a dry run", health, webtest.TestVersion)
	}
}

// forgeKindOnly is the one setting of the configuration's answer the page
// holds to the contract's enum, where api.Config reads any string.
type forgeKindOnly struct {
	Forge struct {
		Kind string `json:"kind"`
	} `json:"forge"`
}

func TestTheConfigurationNamesItsForgeAsTheContractSpellsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

	// Act
	cfg := read[forgeKindOnly](demo, "/api/config")

	// Assert
	if kind := api.ForgeConfigKind(cfg.Forge.Kind); kind != api.ForgeConfigKindGithub {
		t.Errorf("forge.kind reads %q, want %q", kind, api.ForgeConfigKindGithub)
	}
}

func TestThePullRequestDraftProposesTheCodeOwnersOfTheChanges(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)
	demo.pushed()

	// Act
	draft := read[api.PullRequestDraft](demo, "/api/pull-request/draft")

	// Assert
	want := []string{"acme/platform", lee, sam}
	if got := slices.Sorted(slices.Values(draft.Reviewers)); !slices.Equal(got, want) {
		t.Errorf("the draft proposes %v, want %v", got, want)
	}
}

func TestTheAnnouncementPreviewTagsTheCodeOwnersAndTheRepositoryGroups(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t, demo.HoldCI())
	demo.opened()

	// Act
	preview := read[api.Announcement](demo, "/api/announcement")

	// Assert
	tagging := preview.Tagging
	if tagging == nil || !tagging.Available {
		t.Fatalf("the preview offers tagging %+v, want it available", tagging)
	}

	owners := map[string]api.OwnerTagState{}
	for _, owner := range tagging.Owners {
		owners[owner.Owner] = owner.State
	}

	groups := make([]string, 0, len(tagging.Groups))
	for _, group := range tagging.Groups {
		groups = append(groups, group.Slack.Label)
	}

	if owners[sam] != api.OwnerTagStateLinked || owners[lee] != api.OwnerTagStateUnlinked ||
		!slices.Contains(groups, "api-reviewers") {
		t.Errorf("the preview tags owners %v and groups %v, want sam.ortiz linked, lee.chen unlinked, api-reviewers",
			owners, groups)
	}
}

func TestOpeningThePullRequestOffersToLinkTheIssueAndMoveItIntoReview(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

	// Act
	opened := demo.opened()

	// Assert
	actions := make([]api.FollowUpAction, 0, len(opened.FollowUps))
	for _, follow := range opened.FollowUps {
		actions = append(actions, follow.Action)
	}

	want := []api.FollowUpAction{api.FollowUpActionLink, api.FollowUpActionTransition}
	if !slices.Equal(actions, want) {
		t.Errorf("opening offers %v, want %v", actions, want)
	}
}

func TestLinkingThePullRequestToItsIssueSucceeds(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)
	demo.opened()

	// Act
	linked := webtest.Send(t, demo.handler, http.MethodPost, "/api/issues/PROJ-412/link", "")

	// Assert
	if linked.Code != http.StatusOK {
		t.Errorf("linking answered %d, want 200: %s", linked.Code, linked.Body.String())
	}
}

func TestTheOpenedPullRequestCarriesSamsApproval(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)
	demo.opened()
	demo.clock.pass(forgeAskedAgainOnTheWeb)

	// Act
	frame := demo.frame()

	// Assert
	if frame.Review.Pull == nil || frame.Review.Pull.Approvals != 1 {
		t.Errorf("the review reads %+v, want #128 with one approval", frame.Review.Pull)
	}
}

func TestHeldCIKeepsEndToEndRunningWhileTheOtherChecksPass(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t, demo.HoldCI())
	demo.opened()
	demo.clock.pass(time.Hour)

	// Act
	ci := demo.frame().Review.Ci

	// Assert
	if ci == nil || ci.State != api.CIStateRunning || ci.Done != 3 || ci.Total != 4 {
		t.Errorf("an hour after the open CI reads %+v, want three of four passed and still running", ci)
	}
}

// workedDays are the days of the demo's month the Summary shows work on
// before its own day: each weekday whose share of the day's work is not
// nothing, as the web mock's month (mockActivity.ts); the days the demo's
// pending tasks were last changed, a Sunday among them; and yesterday.
func workedDays() []string {
	return []string{
		"2026-09-01", "2026-09-02", "2026-09-04", "2026-09-07", "2026-09-09",
		"2026-09-10", "2026-09-11", "2026-09-13", "2026-09-14", "2026-09-15",
	}
}

func TestTheSummaryMonthShowsWorkOnTheBusierWeekdays(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

	// Act
	activity := read[api.Activity](demo, "/api/activity?from=2026-09-01&to=2026-09-15")

	// Assert
	var days []string

	for _, year := range activity.Years {
		for _, month := range year.Months {
			for _, day := range month.Days {
				days = append(days, day.Date)
			}
		}
	}

	if !slices.Equal(days, workedDays()) {
		t.Errorf("the month shows work on %v, want %v", days, workedDays())
	}
}

func TestTheSummaryShowsEveryKindOfWorkOnAFullDay(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

	// Act
	activity := read[api.Activity](demo, "/api/activity?from=2026-09-10&to=2026-09-10")

	// Assert
	sources := map[string]bool{}

	for _, year := range activity.Years {
		for _, month := range year.Months {
			for _, day := range month.Days {
				for _, hour := range day.Hours {
					for _, item := range hour.Items {
						sources[string(item.Source)] = true
					}
				}
			}
		}
	}

	if len(sources) != 4 {
		t.Errorf("the 10th shows work from %v, want git, tasks, Jira and the forge", sources)
	}
}

func TestPeopleListsTheBranchOwnersOnlyOnceItHasCommits(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

	// Act
	onMain := read[api.People](demo, "/api/people")

	// Assert
	if slices.ContainsFunc(onMain.Owners, func(owner api.OwnerTag) bool { return owner.Owner == lee }) {
		t.Errorf("on main people lists %+v, want no undecided owner of a change", onMain.Owners)
	}
}

func TestPeopleListsTheUndecidedOwnerOfTheChangesOnTheBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)
	demo.committed()

	// Act
	people := read[api.People](demo, "/api/people")

	// Assert
	if !slices.ContainsFunc(people.Owners, func(owner api.OwnerTag) bool {
		return owner.Owner == lee && owner.State == api.OwnerTagStateUnlinked
	}) {
		t.Errorf("people lists %+v, want lee.chen undecided", people.Owners)
	}
}

func TestLinkingAnOwnerKeepsTheLink(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

	// Act
	demo.send(http.MethodPut, "/api/people", `{"owner":"lee.chen","not_on_slack":false,"slack_id":"U0LEE"}`)

	// Assert
	people := read[api.People](demo, "/api/people")
	if !slices.ContainsFunc(people.Owners, func(owner api.OwnerTag) bool {
		return owner.Owner == lee && owner.State == api.OwnerTagStateLinked
	}) {
		t.Errorf("people lists %+v, want lee.chen linked", people.Owners)
	}
}

func TestRelinkingAnOwnerReplacesWhatWasDecided(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

	// Act
	demo.send(http.MethodPut, "/api/people", `{"owner":"sam.ortiz","not_on_slack":true}`)

	// Assert
	people := read[api.People](demo, "/api/people")

	var states []api.OwnerTagState

	for _, owner := range people.Owners {
		if owner.Owner == sam {
			states = append(states, owner.State)
		}
	}

	if !slices.Equal(states, []api.OwnerTagState{api.OwnerTagStateNotOnSlack}) {
		t.Errorf("sam.ortiz is %v, want only not on Slack", states)
	}
}

func TestForgettingAnOwnerAsksAboutThemAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

	// Act
	demo.send(http.MethodDelete, "/api/people?owner=sam.ortiz", "")

	// Assert
	people := read[api.People](demo, "/api/people")
	if slices.ContainsFunc(people.Owners, func(owner api.OwnerTag) bool { return owner.Owner == sam }) {
		t.Errorf("people lists %+v, want sam.ortiz forgotten", people.Owners)
	}
}

func TestChoosingTheGroupsOfTheRepositoryKeepsThem(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

	// Act
	demo.send(http.MethodPut, "/api/repo-groups", `{"ids":["S0PLAT"]}`)

	// Assert
	groups := read[api.RepoGroups](demo, "/api/repo-groups")
	if len(groups.Groups) != 1 || groups.Groups[0].Label != "platform" || groups.Repository != "acme/api" {
		t.Errorf("acme/api's groups are %+v, want platform alone", groups)
	}
}

// itemsOf is every item an activity holds, in the order it reads.
func itemsOf(activity api.Activity) []api.ActivityItem {
	var items []api.ActivityItem

	for _, year := range activity.Years {
		for _, month := range year.Months {
			for _, day := range month.Days {
				for _, hour := range day.Hours {
					items = append(items, hour.Items...)
				}
			}
		}
	}

	return items
}

func TestNoTaskIsAddedAfterItWasCompleted(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

	// Act
	activity := read[api.Activity](demo, "/api/activity?from=2026-09-01&to=2026-09-16")

	// Assert
	added, completed := map[string]time.Time{}, map[string]time.Time{}

	for _, item := range itemsOf(activity) {
		switch item.Verb {
		case "added task":
			added[item.Ref] = item.At
		case "completed task":
			completed[item.Ref] = item.At
		}
	}

	if len(completed) == 0 {
		t.Fatal("the month shows no task completed, want the redaction plan's")
	}

	for ref, done := range completed {
		if entered, isAdded := added[ref]; isAdded && entered.After(done) {
			t.Errorf("task %s was added at %v, after it was completed at %v", ref, entered, done)
		}
	}
}
