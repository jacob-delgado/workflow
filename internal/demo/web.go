// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// webStreamInterval is how often the demo's page is pushed a snapshot when
// nothing was written: each write pushes a frame of its own, so this second
// only paces CI's checks finishing in a recording.
const webStreamInterval = time.Second

// workspace is the demo's Slack workspace.
const workspace = "T0ACME"

// Web is the world as the web server asks of it: the seams the terminal runs
// over, the code owners its pull request proposes as reviewers among them,
// and those only the page shows — whom the code owners are on Slack,
// the groups an announcement may tag, the issue's follow-ups once a pull
// request is open, Sam's approval of it, and a month of work behind the
// Summary's calendar — which the terminal's recording leaves out, so its
// story is told as it was. CI is asked about as often as the terminal asks,
// and version is what the page names the build.
func (w *World) Web(version string) webserver.World {
	cfg := w.Config()
	cfg.Timing.CIInterval = ciInterval.String()
	cfg.Jira.ReviewStatus = statusInReview

	deps := cli.WebDeps(w.Deps())
	w.webJira(&deps.Jira)
	w.webGit(&deps.Git)
	w.webForge(&deps.Forge)
	webSlack(&deps.Messaging)
	newPeople().keep(&deps.Store)
	deps.Tasks.Touched = monthsTouched

	info := webserver.Info{
		Version: version, ForgeKind: forge.KindGitHub, Repository: forgeRepository,
		StreamInterval: webStreamInterval, Taskwarrior: cfg.Taskwarrior,
	}

	return webserver.World{Deps: deps, Config: cfg, Info: info}
}

// webJira links a pull request on its issue, which the demo's Jira takes
// without keeping, and tells the month's moves with the day's.
func (w *World) webJira(seam *seams.Jira) {
	seam.LinkPullRequest = func(context.Context, jira.Key, string, string) error { return nil }
	seam.Activity = func(_ context.Context, start, end time.Time) (jira.Activity, error) {
		return jiraDuring(append(monthsMoves(), w.jiraEvents()...), start, end), nil
	}
}

// webGit tells the month's commits with the day's.
func (w *World) webGit(seam *seams.Git) {
	seam.CommitsBetween = func(_ context.Context, start, end time.Time) []loop.RepositoryCommits {
		return commitsDuring(append(monthsCommits(), w.datedCommits()...), start, end)
	}
}

// webForge has Sam approve the pull request opened, and tells the month's
// reviews with the day's.
func (w *World) webForge(seam *seams.Forge) {
	seam.FindPullRequest = func(_ context.Context, branch string) (forge.PullRequest, bool, error) {
		pull, found := w.pullFor(branch)
		if found {
			pull.Approvals = 1
		}

		return pull, found, nil
	}
	seam.Activity = func(_ context.Context, start, end time.Time) (forge.Activity, error) {
		return forgeDuring(append(monthsReviews(), w.forgeEvents()...), start, end), nil
	}
}

// The demo's people and user groups on Slack.
func slackAna() loop.SlackTarget      { return loop.SlackTarget{ID: "U0ANA", Label: "Ana Lopez"} }
func slackSam() loop.SlackTarget      { return loop.SlackTarget{ID: "U0SAM", Label: "Sam Ortiz"} }
func slackLee() loop.SlackTarget      { return loop.SlackTarget{ID: "U0LEE", Label: "Lee Chen"} }
func groupPlatform() loop.SlackTarget { return loop.SlackTarget{ID: "S0PLAT", Label: "platform"} }
func groupReviewers() loop.SlackTarget {
	return loop.SlackTarget{ID: "S0API", Label: "api-reviewers"}
}

// webSlack is the demo's Slack directory: who is in each channel, its user
// groups, and its workspace.
func webSlack(seam *seams.Messaging) {
	seam.ChannelMembers = func(_ context.Context, named string) ([]loop.SlackTarget, error) {
		members := map[string][]loop.SlackTarget{
			channel:  {slackAna(), slackSam(), slackLee()},
			releases: {slackAna(), slackSam()},
		}

		return members[named], nil
	}
	seam.UserGroups = func(context.Context) ([]loop.SlackTarget, error) {
		return []loop.SlackTarget{groupPlatform(), groupReviewers()}, nil
	}
	seam.Workspace = func(context.Context) (string, error) { return workspace, nil }
}

// people is what the store keeps of the demo's Slack: whom each code owner is
// there, and the groups api's announcements may tag. Sam and the platform
// team are linked and Lee is left to decide, as the page shows an owner
// nobody has linked.
type people struct {
	mu     sync.Mutex
	links  []loop.OwnerLink
	groups []loop.SlackTarget
}

func newPeople() *people {
	return &people{
		links: []loop.OwnerLink{
			{Owner: "sam.ortiz", OnSlack: true, Slack: slackSam()},
			{Owner: "acme/platform", Team: true, OnSlack: true, Slack: groupPlatform()},
		},
		groups: []loop.SlackTarget{groupReviewers()},
	}
}

// keep has the store keep what people holds, for any workspace, the demo
// having one; it records no groups last tagged, so each announcement offers
// the repository's own.
func (p *people) keep(seam *seams.Store) {
	seam.OwnerLinks = func(context.Context, string) ([]loop.OwnerLink, error) {
		return lockedPeople(p, func() []loop.OwnerLink { return slices.Clone(p.links) }), nil
	}
	seam.LinkOwner = func(_ context.Context, _ string, decision loop.OwnerLink) error {
		p.link(decision)

		return nil
	}
	seam.ForgetOwner = func(_ context.Context, _, owner string) error {
		p.forget(owner)

		return nil
	}
	seam.RepoGroups = func(context.Context, string) ([]loop.SlackTarget, error) {
		return lockedPeople(p, func() []loop.SlackTarget { return slices.Clone(p.groups) }), nil
	}
	seam.SetRepoGroups = func(_ context.Context, _ string, groups []loop.SlackTarget) error {
		p.setGroups(groups)

		return nil
	}
	seam.LastGroups = func(context.Context, string) ([]string, bool) { return nil, false }
	seam.RecordGroups = func(context.Context, string, []string) error { return nil }
}

// link keeps decision for its owner, in place of what was decided before.
func (p *people) link(decision loop.OwnerLink) {
	p.mu.Lock()
	defer p.mu.Unlock()

	index := slices.IndexFunc(p.links, func(link loop.OwnerLink) bool { return link.Owner == decision.Owner })
	if index < 0 {
		p.links = append(p.links, decision)

		return
	}

	p.links[index] = decision
}

// forget drops what was decided for owner.
func (p *people) forget(owner string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.links = slices.DeleteFunc(p.links, func(link loop.OwnerLink) bool { return link.Owner == owner })
}

// setGroups makes groups the ones api's announcements may tag.
func (p *people) setGroups(groups []loop.SlackTarget) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.groups = slices.Clone(groups)
}

// lockedPeople runs read under people's lock.
func lockedPeople[T any](p *people, read func() T) T {
	p.mu.Lock()
	defer p.mu.Unlock()

	return read()
}

// The four things of a working day, in the order a day short of all four
// does them: a commit, a task finished, an issue moved and a pull request
// reviewed.
const (
	committing = iota
	finishing
	moving
	reviewing
)

// workload is how many of a working day's four things are done on a weekday,
// by its day of the month, as the web mock's month does them
// (mockActivity.ts): busy days and quiet ones, so the calendar is tinted at
// every level.
func workload(day time.Time) int {
	return [...]int{4, 1, 3, 0, 2}[day.Day()%5]
}

// monthDoing is the weekdays of the demo's month before yesterday on which
// thing was done, each at its midnight.
func monthDoing(thing int) []time.Time {
	start := epoch()
	first := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)

	var days []time.Time

	for day := first; day.Before(yesterday(0, 0)); day = day.AddDate(0, 0, 1) {
		weekend := day.Weekday() == time.Saturday || day.Weekday() == time.Sunday
		if !weekend && workload(day) > thing {
			days = append(days, day)
		}
	}

	return days
}

// at is a time of day on day.
func at(day time.Time, hour, minute int) time.Time {
	return day.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
}

// monthsSubjects are the month's commit subjects, taken in turn.
func monthsSubjects() []string {
	return []string{
		"feat(config): read the forge token from the keychain",
		"fix(wiring): close the request log on shutdown",
		"test(config): cover a token in a nested header",
		"refactor(wiring): name the request log's fields",
		"docs(config): say which keys are redacted",
	}
}

// monthsCommits is a commit on each day of the month that had one.
func monthsCommits() []gitrepo.DatedCommit {
	days := monthDoing(committing)
	subjects := monthsSubjects()
	commits := make([]gitrepo.DatedCommit, 0, len(days))

	for index, day := range days {
		short := fmt.Sprintf("%07x", 0xa1c0000+day.Day())
		commits = append(commits, gitrepo.DatedCommit{
			Hash: short + strings.Repeat("0", 33), Short: short,
			Subject: subjects[index%len(subjects)], Authored: at(day, 10, 15),
		})
	}

	return commits
}

// monthsTouched is the tasks changed since a time: the demo's own, and one
// finished on each day of the month that finished one.
func monthsTouched(_ context.Context, since time.Time) ([]taskwarrior.Task, error) {
	days := monthDoing(finishing)
	finished := make([]taskwarrior.Task, 0, len(days))

	for _, day := range days {
		done := at(day, 11, 40)
		finished = append(finished, taskwarrior.Task{
			UUID:        fmt.Sprintf("5d1f0c2a-7e3b-4a96-8c4d-%012d", day.Day()),
			Description: "Answer the review on the redaction plan", Status: taskwarrior.Completed,
			Project: taskProject, Entry: day, Modified: done, End: done,
		})
	}

	return changedSince(append(finished, allTasks()...), since), nil
}

// monthsMoves is an issue moved into review on each day of the month that
// moved one, the demo's issues taken in turn but for PROJ-412, which no one
// has started before the demo's day.
func monthsMoves() []jira.Event {
	issues := slices.DeleteFunc(issueList(), func(issue jira.Issue) bool { return issue.Key == workedKey })
	days := monthDoing(moving)
	moves := make([]jira.Event, 0, len(days))

	for index, day := range days {
		issue := issues[index%len(issues)]
		moves = append(moves, jira.Event{
			At: at(day, 14, 30), Kind: jira.EventMoved, Key: issue.Key, Summary: issue.Summary, Detail: statusInReview,
		})
	}

	return moves
}

// monthsReviews is a pull request reviewed on each day of the month that
// reviewed one, the review queue's taken in turn.
func monthsReviews() []forge.Event {
	queue := reviewQueue()
	days := monthDoing(reviewing)
	reviews := make([]forge.Event, 0, len(days))

	for index, day := range days {
		pull := queue[index%len(queue)]
		reviews = append(reviews, forge.Event{
			At: at(day, 15, 50), Kind: forge.EventReviewed, Number: pull.Number, Title: pull.Title, URL: pull.URL,
			Repository: pull.Repository,
		})
	}

	return reviews
}
