// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package demo is a made-up day's work for the docs' recordings: Jira, the
// forge, Slack, git, Taskwarrior and the directories faked as one world the
// real terminal interface runs over, as tui.Deps, and the real web server
// over, as a webserver.World with the seams only the page shows. It tells the
// story the web's mock tells (web/src/dev/mock*.ts): Ana Lopez, in acme's api
// repository, on PROJ-412, announcing to #dev-workflow.
//
// Trade-off TRADE-57: this world and internal/tui's test world are two fakes
// of one world, kept apart because they answer to different readers.
package demo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// ciInterval is how often the demo's interface asks about CI while it runs:
// twice in the four seconds its checks take, so a recording sees CI move on
// before it passes.
const ciInterval = 2 * time.Second

// The people, places and services of the demo's story.
const (
	jiraBase = "https://jira.example.com"
	// githubHost is where every repository of the story lives.
	githubHost = "github.com"
	channel    = "#dev-workflow"
	// releases is the other channel the demo's Slack has.
	releases = "#releases"
	author   = "ana.lopez"
	home     = "/home/ana"
	apiRoot  = home + "/src/api"
	webRoot  = home + "/src/web"
)

// epoch is when the demo's day stands at its start: four in the afternoon,
// in UTC, so the Summary's calendar is the same whatever zone it runs in.
func epoch() time.Time {
	return time.Date(2026, time.September, 16, 16, 0, 0, 0, time.UTC)
}

// World is everything outside the terminal, made up, and what has happened to
// it since it began. Every seam reads and changes it under one lock, since
// the interface asks from several commands at once.
type World struct {
	mu sync.Mutex
	// elapsed is how long the demo has run, which the clock adds to epoch.
	elapsed func() time.Duration

	issues   []jira.Issue
	position map[jira.Key]int
	repo     repository
	// pulls are the numbers of the pull requests opened, by the branch each
	// is for, and opened is each of them, by its number.
	pulls     map[string]int
	opened    map[int]openedPull
	announced []loop.Announced
	// moves are the issues moved since the demo began, for the Summary.
	moves []jira.Event
	// scope is the commit scope last used.
	scope string
	// readLatency is how long the slow reads take; see ReadLatency.
	readLatency time.Duration
	// pushLatency is how long a push takes once it has printed; see
	// PushLatency.
	pushLatency time.Duration
	// holdCI keeps the end-to-end check running; see HoldCI.
	holdCI bool
}

// Option is a way the world can be made other than its default.
type Option func(*World)

// ReadLatency makes the reads the interface shows itself waiting on — the
// branches origin holds, and a pull request's CI — take latency before they
// answer, as a real remote does, so a recording holds each "reading" state for
// a beat rather than a frame. Without it they answer at once, as the tests
// want.
func ReadLatency(latency time.Duration) Option {
	return func(w *World) { w.readLatency = latency }
}

// PushLatency makes a push take latency to end once its hooks' lines and
// the remote's answer are printed, as a push waits on its remote, so a
// recording holds the push's output for a beat rather than a frame. Stopping
// the push ends it sooner. Without it a push ends at once, as the tests want.
func PushLatency(latency time.Duration) Option {
	return func(w *World) { w.pushLatency = latency }
}

// HoldCI keeps every pull request's end-to-end check running for good while
// the others pass on their schedule, so a screenshot finds CI three checks of
// four through, and an announcement still able to wait for it, however long
// the page takes to get there.
func HoldCI() Option {
	return func(w *World) { w.holdCI = true }
}

// openedPull is a pull request and when it was opened, which its CI counts
// from.
type openedPull struct {
	pull forge.PullRequest
	at   time.Time
}

// New is the world at the start of the demo, its clock elapsed on from the
// demo's epoch and changed by options: the repository on main with the
// morning's changes not yet committed, and PROJ-412 still to do, its task not
// started.
func New(elapsed func() time.Duration, options ...Option) *World {
	issues := issueList()
	position := make(map[jira.Key]int, len(issues))

	for index, issue := range issues {
		position[issue.Key] = index
	}

	world := &World{
		elapsed: elapsed, issues: issues, position: position,
		repo: newRepository(), pulls: map[string]int{}, opened: map[int]openedPull{}, scope: "config",
	}
	for _, option := range options {
		option(world)
	}

	return world
}

// Config is the configuration the demo runs under: Jira, Slack and GitHub set
// up, credentials made up, and motion on for the recording.
func (w *World) Config() config.Config {
	cfg := config.Default()
	cfg.Jira.BaseURL, cfg.Jira.Token, cfg.Jira.Project = jiraBase, "demo-token", "PROJ"
	cfg.Messaging.Kind, cfg.Messaging.ClientID, cfg.Messaging.Channel = config.KindSlack, "1234.5678", channel
	cfg.Messaging.Channels = []string{channel, releases}
	cfg.Forge.Kind, cfg.Forge.Host = "github", githubHost
	cfg.Path = apiRoot + "/.workflow.json"
	cfg.Files = config.Files{Repo: cfg.Path, Home: home + "/.workflow.json"}

	return cfg
}

// Deps is the world as the interface asks of it. What the demo never shows —
// the editor, lefthook, the settings files, a browser, the clipboard, a
// desktop notification — is left nil, so it is never offered.
func (w *World) Deps() tui.Deps {
	return tui.Deps{
		Jira: w.jiraDeps(), Git: w.gitDeps(), Forge: w.forgeDeps(), Messaging: w.messagingDeps(),
		Store: w.storeDeps(), Tasks: w.taskDeps(), Repositories: w.repositoriesDeps(),
		Clock: w.now, CIInterval: ciInterval,
	}
}

// now is the demo's clock: its epoch, moved on by how long it has run.
func (w *World) now() time.Time {
	return epoch().Add(w.elapsed())
}

// slowly waits out the world's read latency before a slow read answers, or
// until ctx ends. It waits without the lock, so every other seam answers
// meanwhile.
func (w *World) slowly(ctx context.Context) error {
	wait := time.NewTimer(w.readLatency)
	defer wait.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("the demo's read: %w", ctx.Err())
	case <-wait.C:
		return nil
	}
}

// locked runs read under the world's lock.
func locked[T any](w *World, read func() T) T {
	w.mu.Lock()
	defer w.mu.Unlock()

	return read()
}
